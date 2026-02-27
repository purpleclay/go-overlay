package mod

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
	semver "github.com/Masterminds/semver/v3"
	"github.com/sourcegraph/conc/pool"
)

const (
	goModFile  = "go.mod"
	goWorkFile = "go.work"
	vendorFile = "govendor.toml"
)

type vendorOptions struct {
	detectDrift     bool
	force           bool
	paths           []string
	recursive       bool
	maxDepth        int
	extraPlatforms  []string
	workspace       bool
	vendoredVersion string
	strict          bool
}

type VendorOption func(*vendorOptions)

func WithDriftDetection() VendorOption {
	return func(opts *vendorOptions) {
		opts.detectDrift = true
	}
}

func WithForce() VendorOption {
	return func(opts *vendorOptions) {
		opts.force = true
	}
}

func WithPaths(paths ...string) VendorOption {
	return func(opts *vendorOptions) {
		for _, path := range paths {
			if base := filepath.Base(path); base == goModFile || base == goWorkFile || base == vendorFile {
				path = filepath.Dir(path)
			}
			opts.paths = append(opts.paths, path)
		}
	}
}

func WithRecursive(maxDepth int) VendorOption {
	return func(opts *vendorOptions) {
		opts.recursive = true
		opts.maxDepth = maxDepth
	}
}

func WithWorkspace() VendorOption {
	return func(opts *vendorOptions) {
		opts.workspace = true
	}
}

func WithIncludePlatforms(platforms []string) VendorOption {
	return func(opts *vendorOptions) {
		opts.extraPlatforms = platforms
	}
}

func WithVendoredVersion(version string) VendorOption {
	return func(opts *vendorOptions) {
		opts.vendoredVersion = version
	}
}

func WithStrict() VendorOption {
	return func(opts *vendorOptions) {
		opts.strict = true
	}
}

type Vendor struct {
	opts vendorOptions
}

func NewVendor(opts ...VendorOption) *Vendor {
	v := &Vendor{}
	for _, opt := range opts {
		opt(&v.opts)
	}
	return v
}

var errVendorFailed = fmt.Errorf("vendor failed")

func (v *Vendor) VendorFiles() error {
	if v.opts.workspace {
		return v.processWorkspaceMode()
	}

	if !v.opts.recursive {
		if goWork := v.findWorkspace(); goWork != nil {
			return v.processWorkspace(goWork)
		}
	}

	modFiles, missingResults := v.findModFiles()

	p := pool.NewWithResults[vendorResult]()
	for _, modFile := range modFiles {
		p.Go(func() vendorResult {
			return v.processModFile(modFile)
		})
	}

	results := append(missingResults, p.Wait()...)

	sort.Slice(results, func(i, j int) bool {
		return results[i].path < results[j].path
	})

	fmt.Println(renderResultsTable(results))

	for _, r := range results {
		if r.status.IsFailure() {
			return errVendorFailed
		}
	}

	return nil
}

func (v *Vendor) findWorkspace() *GoWorkFile {
	path := "."
	if len(v.opts.paths) > 0 {
		path = v.opts.paths[0]
	}

	workPath := filepath.Join(path, goWorkFile)
	if _, err := os.Stat(workPath); err == nil {
		goWork, err := ParseGoWorkFile(workPath)
		if err == nil {
			return goWork
		}
	}

	vendorPath := filepath.Join(path, vendorFile)
	data, err := os.ReadFile(vendorPath)
	if err != nil {
		return nil
	}

	var manifest struct {
		Workspace *WorkspaceConfig `toml:"workspace"`
	}
	if err := toml.Unmarshal(data, &manifest); err != nil || manifest.Workspace == nil {
		return nil
	}

	goWork, err := NewGoWorkFileFromManifest(path, manifest.Workspace)
	if err != nil {
		return nil
	}

	return goWork
}

func (v *Vendor) processWorkspace(goWork *GoWorkFile) error {
	result := v.processWorkspaceManifest(goWork)

	fmt.Println(renderResultsTable([]vendorResult{result}))

	if result.status.IsFailure() {
		return errVendorFailed
	}

	return nil
}

func (v *Vendor) processWorkspaceManifest(goWork *GoWorkFile) vendorResult {
	displayPath := filepath.Join(goWork.Dir(), goWorkFile)

	vendorPath := filepath.Join(goWork.Dir(), vendorFile)
	existingData, err := os.ReadFile(vendorPath)

	extraPlatforms := v.opts.extraPlatforms

	if os.IsNotExist(err) {
		if v.opts.detectDrift {
			return resultMissing(displayPath)
		}
	} else if err != nil {
		return resultError(displayPath, err)
	} else {
		if result, drifted := v.checkDrift(displayPath, existingData, goWork.Hash()); drifted {
			if v.opts.detectDrift || result.status == statusWarning {
				return result
			}
		} else {
			if len(extraPlatforms) == 0 {
				if existingPlatforms, err := extractPlatforms(existingData); err == nil {
					extraPlatforms = existingPlatforms
				}
			}
			if !v.opts.force {
				if v.opts.detectDrift || len(v.opts.extraPlatforms) == 0 {
					return resultOK(displayPath)
				}
			}
		}
	}

	platforms := append(DefaultPlatforms, extraPlatforms...)
	depCount, err := v.generateWorkspaceManifest(goWork, platforms, extraPlatforms)
	if err != nil {
		return resultError(displayPath, err)
	}

	return resultGenerated(displayPath, depCount)
}

func (v *Vendor) generateWorkspaceManifest(goWork *GoWorkFile, platforms []string, includePlatforms []string) (int, error) {
	manifest, err := newWorkspaceManifest(goWork, platforms, includePlatforms, v.opts.vendoredVersion)
	if err != nil {
		return 0, err
	}

	var buf bytes.Buffer
	if _, err := manifest.WriteTo(&buf); err != nil {
		return 0, err
	}

	outputPath := filepath.Join(goWork.Dir(), vendorFile)
	if err := os.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
		return 0, err
	}

	return len(manifest.Mod), nil
}

func (v *Vendor) findModFiles() (modFiles []string, missing []vendorResult) {
	paths := v.opts.paths
	if len(paths) == 0 {
		paths = []string{"."}
	}

	if v.opts.recursive {
		p := pool.NewWithResults[[]string]().WithErrors()
		for _, path := range paths {
			p.Go(func() ([]string, error) {
				scanner := NewFileTreeScanner(WithMaxDepth(v.opts.maxDepth))
				return scanner.ScanFrom(path)
			})
		}

		results, err := p.Wait()
		if err != nil {
			return nil, nil
		}

		for _, found := range results {
			modFiles = append(modFiles, found...)
		}

		if len(modFiles) == 0 {
			for _, path := range paths {
				missing = append(missing, resultNotFound(filepath.Join(path, goModFile)))
			}
		}

		return modFiles, missing
	}

	for _, path := range paths {
		modPath := filepath.Join(path, goModFile)
		if _, err := os.Stat(modPath); err != nil {
			missing = append(missing, resultNotFound(modPath))
		} else {
			modFiles = append(modFiles, modPath)
		}
	}

	return modFiles, missing
}

func (v *Vendor) processModFile(path string) vendorResult {
	goMod, err := ParseGoModFile(path)
	if err != nil {
		return resultError(path, err)
	}

	if !goMod.HasDependencies() {
		return resultSkipped(path)
	}

	vendorPath := filepath.Join(goMod.Dir(), vendorFile)
	existingData, err := os.ReadFile(vendorPath)

	extraPlatforms := v.opts.extraPlatforms

	if os.IsNotExist(err) {
		if v.opts.detectDrift {
			return resultMissing(path)
		}
	} else if err != nil {
		return resultError(path, err)
	} else {
		if result, drifted := v.checkDrift(path, existingData, goMod.Hash()); drifted {
			if v.opts.detectDrift || result.status == statusWarning {
				return result
			}
		} else {
			if len(extraPlatforms) == 0 {
				if existingPlatforms, err := extractPlatforms(existingData); err == nil {
					extraPlatforms = existingPlatforms
				}
			}
			if !v.opts.force {
				if v.opts.detectDrift || len(v.opts.extraPlatforms) == 0 {
					return resultOK(path)
				}
			}
		}
	}

	platforms := append(DefaultPlatforms, extraPlatforms...)
	depCount, err := v.generateManifest(goMod, platforms, extraPlatforms)
	if err != nil {
		return resultError(path, err)
	}

	return resultGenerated(path, depCount)
}

func (v *Vendor) generateManifest(goMod *GoModFile, platforms []string, includePlatforms []string) (int, error) {
	manifest, err := newManifest(goMod, platforms, includePlatforms, v.opts.vendoredVersion)
	if err != nil {
		return 0, err
	}

	var buf bytes.Buffer
	if _, err := manifest.WriteTo(&buf); err != nil {
		return 0, err
	}

	outputPath := filepath.Join(goMod.Dir(), vendorFile)
	if err := os.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
		return 0, err
	}

	return len(manifest.Mod), nil
}

func (v *Vendor) processWorkspaceMode() error {
	path := "."
	if len(v.opts.paths) > 0 {
		path = v.opts.paths[0]
	}

	manifestPath, err := FindWorkspaceManifest(path)
	if err != nil {
		return err
	}

	var result vendorResult
	if manifestPath == "" {
		result = resultMissing(filepath.Join(path, vendorFile))
	} else {
		manifestDir := filepath.Dir(manifestPath)
		goWork := v.findWorkspaceAt(manifestDir)
		if goWork == nil {
			result = resultError(manifestPath, fmt.Errorf("invalid workspace manifest"))
		} else {
			result = v.processWorkspaceManifest(goWork)
		}
	}

	fmt.Println(renderResultsTable([]vendorResult{result}))

	if result.status.IsFailure() {
		return errVendorFailed
	}
	return nil
}

func (v *Vendor) findWorkspaceAt(path string) *GoWorkFile {
	workPath := filepath.Join(path, goWorkFile)
	if _, err := os.Stat(workPath); err == nil {
		goWork, err := ParseGoWorkFile(workPath)
		if err == nil {
			return goWork
		}
	}

	vendorPath := filepath.Join(path, vendorFile)
	data, err := os.ReadFile(vendorPath)
	if err != nil {
		return nil
	}

	var manifest struct {
		Workspace *WorkspaceConfig `toml:"workspace"`
	}
	if err := toml.Unmarshal(data, &manifest); err != nil || manifest.Workspace == nil {
		return nil
	}

	goWork, err := NewGoWorkFileFromManifest(path, manifest.Workspace)
	if err != nil {
		return nil
	}

	return goWork
}

func (v *Vendor) checkDrift(path string, data []byte, currentHash string) (vendorResult, bool) {
	manifestSchema, err := extractSchema(data)
	if err != nil {
		return resultError(path, err), true
	}
	if manifestSchema != schemaVersion {
		return resultSchemaMismatch(path, manifestSchema, schemaVersion), true
	}

	var reasons []string
	hasDrift := false
	isWarning := false

	if v.opts.vendoredVersion != "" && !v.opts.force {
		manifest, err := extractVersion(data)
		if err != nil {
			return resultError(path, err), true
		}
		if manifest != "" && manifest != v.opts.vendoredVersion {
			mv, err1 := semver.NewVersion(manifest)
			cv, err2 := semver.NewVersion(v.opts.vendoredVersion)
			if err1 != nil || err2 != nil {
				reasons = append(reasons, fmt.Sprintf("govendor version mismatch: %s → %s (use --check --strict to enforce)", manifest, v.opts.vendoredVersion))
				isWarning = true
			} else if mv.Major() != cv.Major() {
				reasons = append(reasons, fmt.Sprintf("govendor version mismatch: %s → %s (incompatible major version)", manifest, v.opts.vendoredVersion))
				hasDrift = true
			} else if v.opts.strict {
				reasons = append(reasons, fmt.Sprintf("govendor version mismatch: %s → %s", manifest, v.opts.vendoredVersion))
				hasDrift = true
			} else {
				reasons = append(reasons, fmt.Sprintf("govendor version mismatch: %s → %s (use --check --strict to enforce)", manifest, v.opts.vendoredVersion))
				isWarning = true
			}
		}
	}

	existingHash, err := extractHash(data)
	if err != nil {
		return resultError(path, err), true
	}
	if existingHash != currentHash {
		ft := fileType(path)
		reasons = append(reasons, fmt.Sprintf("hash: %s has changed\n      %-14s %s\n      %-14s %s", ft, ft+":", currentHash, "govendor.toml:", existingHash))
		hasDrift = true
	}

	if len(reasons) == 0 {
		return vendorResult{}, false
	}

	if hasDrift {
		return resultDriftDetected(path, reasons), true
	}
	if isWarning {
		return resultVersionWarning(path, reasons), true
	}
	return vendorResult{}, false
}

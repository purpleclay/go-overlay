package mod

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/purpleclay/go-overlay/internal/vendor"
	"github.com/sourcegraph/conc/pool"
)

const (
	goModFile  = "go.mod"
	goWorkFile = "go.work"
	vendorFile = "govendor.toml"
)

type vendorOptions struct {
	detectDrift    bool
	force          bool
	paths          []string
	recursive      bool
	maxDepth       int
	extraPlatforms []string
	workspace      bool
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

// manifestSource is implemented by both GoModFile and GoWorkFile, allowing
// the common drift detection and generation algorithm to be shared.
type manifestSource interface {
	Hash() string
	Dir() string
}

func (v *Vendor) VendorFiles() ([]vendor.Result, error) {
	if v.opts.workspace {
		return v.processWorkspaceMode()
	}

	if !v.opts.recursive {
		path := "."
		if len(v.opts.paths) > 0 {
			path = v.opts.paths[0]
		}
		if goWork := v.findWorkspaceAt(path); goWork != nil {
			return v.toResults(v.processSource(goWork, filepath.Join(goWork.Dir(), goWorkFile)))
		}
	}

	modFiles, missingResults, err := v.findModFiles()
	if err != nil {
		return nil, err
	}

	p := pool.NewWithResults[vendor.Result]()
	for _, modFile := range modFiles {
		p.Go(func() vendor.Result {
			goMod, err := ParseGoModFile(modFile)
			if err != nil {
				return resultError(modFile, err)
			}
			if !goMod.HasDependencies() {
				return resultSkipped(modFile)
			}
			return v.processSource(goMod, modFile)
		})
	}

	results := append(missingResults, p.Wait()...)

	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})

	for _, r := range results {
		if r.Status.IsFailure() {
			return results, errVendorFailed
		}
	}

	return results, nil
}

func (v *Vendor) toResults(r vendor.Result) ([]vendor.Result, error) {
	if r.Status.IsFailure() {
		return []vendor.Result{r}, errVendorFailed
	}
	return []vendor.Result{r}, nil
}

// processSource implements the common drift detection and generation algorithm
// for both GoModFile and GoWorkFile sources.
func (v *Vendor) processSource(src manifestSource, displayPath string) vendor.Result {
	vendorPath := filepath.Join(src.Dir(), vendorFile)
	existingData, err := os.ReadFile(vendorPath)

	extraPlatforms := v.opts.extraPlatforms

	if os.IsNotExist(err) {
		if v.opts.detectDrift {
			return resultMissing(displayPath)
		}
	} else if err != nil {
		return resultError(displayPath, err)
	} else {
		existing, err := vendor.Parse(existingData)
		if err != nil {
			return resultError(displayPath, err)
		}

		if existing.Schema != vendor.SchemaVersion {
			if v.opts.detectDrift {
				return resultSchemaMismatch(displayPath, existing.Schema, vendor.SchemaVersion)
			}
			if len(extraPlatforms) == 0 {
				extraPlatforms = existing.IncludePlatforms
			}
		} else {
			if len(extraPlatforms) == 0 {
				extraPlatforms = existing.IncludePlatforms
			}

			if !v.opts.force && existing.Hash == src.Hash() {
				if v.opts.detectDrift || len(v.opts.extraPlatforms) == 0 {
					return resultOK(displayPath)
				}
			}

			if v.opts.detectDrift {
				return resultDrift(displayPath, src.Hash(), existing.Hash)
			}
		}
	}

	platforms := append(DefaultPlatforms, extraPlatforms...)
	depCount, err := v.generate(src, platforms, extraPlatforms)
	if err != nil {
		return resultError(displayPath, err)
	}

	return resultGenerated(displayPath, depCount)
}

func (v *Vendor) generate(src manifestSource, platforms, includePlatforms []string) (int, error) {
	var deps []vendor.ModuleConfig
	var workspace *vendor.WorkspaceConfig
	var err error

	switch s := src.(type) {
	case *GoModFile:
		deps, err = s.Dependencies(platforms)
	case *GoWorkFile:
		deps, err = s.Dependencies(platforms)
		workspace = s.WorkspaceConfig()
	default:
		return 0, fmt.Errorf("unsupported source type: %T", src)
	}

	if err != nil {
		return 0, err
	}

	m := vendor.New(src.Hash(), deps, includePlatforms, workspace)

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		return 0, err
	}

	outputPath := filepath.Join(src.Dir(), vendorFile)
	if err := os.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
		return 0, err
	}

	return len(m.Mod), nil
}

func (v *Vendor) findModFiles() (modFiles []string, missing []vendor.Result, err error) {
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
			return nil, nil, err
		}

		for _, found := range results {
			modFiles = append(modFiles, found...)
		}

		if len(modFiles) == 0 {
			for _, path := range paths {
				missing = append(missing, resultNotFound(filepath.Join(path, goModFile)))
			}
		}

		return modFiles, missing, nil
	}

	for _, path := range paths {
		modPath := filepath.Join(path, goModFile)
		if _, err := os.Stat(modPath); err != nil {
			missing = append(missing, resultNotFound(modPath))
		} else {
			modFiles = append(modFiles, modPath)
		}
	}

	return modFiles, missing, nil
}

func (v *Vendor) processWorkspaceMode() ([]vendor.Result, error) {
	path := "."
	if len(v.opts.paths) > 0 {
		path = v.opts.paths[0]
	}

	manifestPath, err := FindWorkspaceManifest(path)
	if err != nil {
		return nil, err
	}

	var result vendor.Result
	if manifestPath == "" {
		result = resultMissing(filepath.Join(path, vendorFile))
	} else {
		manifestDir := filepath.Dir(manifestPath)
		goWork := v.findWorkspaceAt(manifestDir)
		if goWork == nil {
			result = resultError(manifestPath, fmt.Errorf("invalid workspace manifest"))
		} else {
			result = v.processSource(goWork, filepath.Join(manifestDir, goWorkFile))
		}
	}

	return v.toResults(result)
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

	existing, err := vendor.Parse(data)
	if err != nil || existing.Workspace == nil {
		return nil
	}

	goWork, err := NewGoWorkFileFromManifest(path, existing.Workspace)
	if err != nil {
		return nil
	}

	return goWork
}

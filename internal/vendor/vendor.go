package vendor

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/purpleclay/go-overlay/internal/mod"
	"github.com/sourcegraph/conc/pool"
)

const vendorFile = "govendor.toml"

type vendorOptions struct {
	detectDrift    bool
	force          bool
	paths          []string
	recursive      bool
	maxDepth       int
	extraPlatforms []string
	workspace      bool
}

type Option func(*vendorOptions)

func WithDriftDetection() Option {
	return func(opts *vendorOptions) {
		opts.detectDrift = true
	}
}

func WithForce() Option {
	return func(opts *vendorOptions) {
		opts.force = true
	}
}

func WithPaths(paths ...string) Option {
	return func(opts *vendorOptions) {
		for _, path := range paths {
			if base := filepath.Base(path); base == mod.GoModFilename || base == mod.GoWorkFilename || base == vendorFile {
				path = filepath.Dir(path)
			}
			opts.paths = append(opts.paths, path)
		}
	}
}

func WithRecursive(maxDepth int) Option {
	return func(opts *vendorOptions) {
		opts.recursive = true
		opts.maxDepth = maxDepth
	}
}

func WithWorkspace() Option {
	return func(opts *vendorOptions) {
		opts.workspace = true
	}
}

func WithIncludePlatforms(platforms []string) Option {
	return func(opts *vendorOptions) {
		opts.extraPlatforms = platforms
	}
}

// Resolver resolves Go module dependencies. The orchestrator delegates all
// toolchain interaction to a Resolver, which is injected at construction
// time. This keeps the vendor package free of process-execution concerns
// and allows the orchestrator to be exercised against fake resolvers.
type Resolver interface {
	ResolveModule(goMod *mod.GoModFile, platforms []string) ([]mod.ModuleConfig, error)
	ResolveWorkspace(goWork *mod.GoWorkFile, platforms []string) ([]mod.ModuleConfig, error)
}

type Vendor struct {
	opts     vendorOptions
	resolver Resolver
}

func NewVendor(resolver Resolver, opts ...Option) *Vendor {
	v := &Vendor{resolver: resolver}
	for _, opt := range opts {
		opt(&v.opts)
	}
	return v
}

var errVendorFailed = fmt.Errorf("vendor failed")

// dependencySource is satisfied by *mod.GoModFile and *mod.GoWorkFile.
// Type switches on this value are used to dispatch to type-specific behaviour.
type dependencySource = any

func (v *Vendor) VendorFiles() ([]Result, error) {
	if v.opts.workspace {
		return v.processWorkspaceMode()
	}

	if !v.opts.recursive {
		path := "."
		if len(v.opts.paths) > 0 {
			path = v.opts.paths[0]
		}
		goWork, err := v.findWorkspaceAt(path)
		if err != nil {
			return nil, err
		}
		if goWork != nil {
			return v.toResults(v.processSource(goWork, filepath.Join(goWork.Dir, mod.GoWorkFilename), goWork.WorkspaceConfig()))
		}
	}

	modFiles, missingResults, err := v.findModFiles()
	if err != nil {
		return nil, err
	}

	p := pool.NewWithResults[Result]()
	for _, modFile := range modFiles {
		p.Go(func() Result {
			goMod, err := mod.ParseGoModFile(modFile)
			if err != nil {
				return resultError(modFile, err)
			}
			if !goMod.HasDependencies() && !goMod.HasTools() {
				return resultSkipped(modFile)
			}
			return v.processSource(goMod, modFile, nil)
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

func (v *Vendor) toResults(r Result) ([]Result, error) {
	if r.Status.IsFailure() {
		return []Result{r}, errVendorFailed
	}
	return []Result{r}, nil
}

// processSource implements the common drift detection and generation algorithm
// for both *mod.GoModFile and *mod.GoWorkFile sources.
func (v *Vendor) processSource(src dependencySource, displayPath string, workspace *mod.WorkspaceConfig) Result {
	dir := filepath.Dir(displayPath)
	vendorPath := filepath.Join(dir, vendorFile)
	existingData, err := os.ReadFile(vendorPath)

	extraPlatforms := v.opts.extraPlatforms

	if os.IsNotExist(err) {
		if v.opts.detectDrift {
			return resultMissing(displayPath)
		}
	} else if err != nil {
		return resultError(displayPath, err)
	} else {
		existing, err := Parse(existingData)
		if err != nil {
			return resultError(displayPath, err)
		}

		if existing.Schema != SchemaVersion {
			if v.opts.detectDrift {
				return resultSchemaMismatch(displayPath, existing.Schema, SchemaVersion)
			}
			if len(extraPlatforms) == 0 {
				extraPlatforms = existing.IncludePlatforms
			}
		} else {
			if len(extraPlatforms) == 0 {
				extraPlatforms = existing.IncludePlatforms
			}

			drifted, err := IsDrifted(src, existing)
			if err != nil {
				return resultError(displayPath, err)
			}
			if !v.opts.force && !drifted {
				if v.opts.detectDrift || len(v.opts.extraPlatforms) == 0 {
					return resultOK(displayPath)
				}
			}

			if v.opts.detectDrift {
				if drifted {
					return resultDrift(displayPath)
				}
				return resultOK(displayPath)
			}
		}
	}

	platforms := append(mod.DefaultPlatforms(), extraPlatforms...)
	depCount, err := v.generate(src, dir, platforms, extraPlatforms, workspace)
	if err != nil {
		return resultError(displayPath, err)
	}

	return resultGenerated(displayPath, depCount)
}

func (v *Vendor) generate(src dependencySource, dir string, platforms, includePlatforms []string, workspace *mod.WorkspaceConfig) (int, error) {
	var deps []mod.ModuleConfig
	var excludes map[string][]string
	var err error

	switch s := src.(type) {
	case *mod.GoModFile:
		deps, err = v.resolver.ResolveModule(s, platforms)
		if len(s.Excludes) > 0 {
			excludes = s.Excludes
		}
	case *mod.GoWorkFile:
		deps, err = v.resolver.ResolveWorkspace(s, platforms)
		if err != nil {
			return 0, err
		}
		members, err := s.ParseMembers()
		if err != nil {
			return 0, err
		}
		merged := make(map[string][]string)
		for _, m := range members {
			for path, versions := range m.Excludes {
				merged[path] = append(merged[path], versions...)
			}
		}
		for path, versions := range merged {
			slices.Sort(versions)
			merged[path] = slices.Compact(versions)
		}
		if len(merged) > 0 {
			excludes = merged
		}
	default:
		return 0, fmt.Errorf("unsupported dependency source: %T", src)
	}
	if err != nil {
		return 0, err
	}

	m := New(deps, includePlatforms, workspace, excludes)

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		return 0, err
	}

	outputPath := filepath.Join(dir, vendorFile)
	if err := os.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
		return 0, err
	}

	return len(m.Mod), nil
}

func (v *Vendor) findModFiles() (modFiles []string, missing []Result, err error) {
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
				missing = append(missing, resultNotFound(filepath.Join(path, mod.GoModFilename)))
			}
		}

		return modFiles, missing, nil
	}

	for _, path := range paths {
		modPath := filepath.Join(path, mod.GoModFilename)
		if _, err := os.Stat(modPath); err != nil {
			missing = append(missing, resultNotFound(modPath))
		} else {
			modFiles = append(modFiles, modPath)
		}
	}

	return modFiles, missing, nil
}

func (v *Vendor) processWorkspaceMode() ([]Result, error) {
	path := "."
	if len(v.opts.paths) > 0 {
		path = v.opts.paths[0]
	}

	manifestPath, err := FindWorkspaceManifest(path)
	if err != nil {
		return nil, err
	}

	var result Result
	if manifestPath == "" {
		result = resultMissing(filepath.Join(path, vendorFile))
	} else {
		manifestDir := filepath.Dir(manifestPath)
		goWork, err := v.findWorkspaceAt(manifestDir)
		if err != nil {
			result = resultError(manifestPath, err)
		} else if goWork == nil {
			result = resultError(manifestPath, fmt.Errorf("invalid workspace manifest"))
		} else {
			result = v.processSource(goWork, filepath.Join(manifestDir, mod.GoWorkFilename), goWork.WorkspaceConfig())
		}
	}

	return v.toResults(result)
}

func (v *Vendor) findWorkspaceAt(path string) (*mod.GoWorkFile, error) {
	workPath := filepath.Join(path, mod.GoWorkFilename)
	if _, err := os.Stat(workPath); err == nil {
		goWork, err := mod.ParseGoWorkFile(workPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", workPath, err)
		}
		return goWork, nil
	}

	vendorPath := filepath.Join(path, vendorFile)
	data, err := os.ReadFile(vendorPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", vendorPath, err)
	}

	existing, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", vendorPath, err)
	}
	if existing.Workspace == nil {
		return nil, nil
	}

	goWork, err := mod.NewGoWorkFileFromManifest(path, existing.Workspace)
	if err != nil {
		return nil, err
	}

	return goWork, nil
}

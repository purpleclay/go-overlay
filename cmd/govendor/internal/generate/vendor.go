package generate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/purpleclay/conker/pool"
	"github.com/purpleclay/go-overlay/internal/gomod"
	"github.com/purpleclay/go-overlay/internal/manifest"
)

type vendorOptions struct {
	detectDrift bool
	paths       []string
	recursive   bool
	maxDepth    int
	workspace   bool
}

type Option func(*vendorOptions)

func WithDriftDetection() Option {
	return func(opts *vendorOptions) {
		opts.detectDrift = true
	}
}

func WithPaths(paths ...string) Option {
	return func(opts *vendorOptions) {
		for _, path := range paths {
			if base := filepath.Base(path); base == gomod.Filename || base == gomod.WorkFilename || base == manifest.Filename {
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

// Resolver resolves Go module dependencies. The orchestrator delegates all
// toolchain interaction to a Resolver, which is injected at construction
// time. This keeps the vendor package free of process-execution concerns
// and allows the orchestrator to be exercised against fake resolvers.
type Resolver interface {
	ResolveModule(ctx context.Context, goMod *gomod.File, existingMods map[string]manifest.Module) ([]manifest.Module, error)
	ResolveWorkspace(ctx context.Context, goWork *gomod.WorkFile, members []gomod.WorkspaceMember, existingMods map[string]manifest.Module) ([]manifest.Module, error)
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

// ErrVendorFailed indicates one or more results in the rendered table failed.
// It carries no information beyond what the table already shows, so callers
// that have rendered the results table may choose to suppress its message.
var ErrVendorFailed = fmt.Errorf("vendor failed")

// dependencySource is satisfied by *gomod.File and *gomod.WorkFile.
// Type switches on this value are used to dispatch to type-specific behaviour.
type dependencySource = any

func (v *Vendor) VendorFiles(ctx context.Context) ([]Result, error) {
	if v.opts.workspace {
		return v.processWorkspaceMode(ctx)
	}

	paths := v.opts.paths
	if len(paths) == 0 {
		paths = []string{"."}
	}

	// Without --recursive, every path is checked for a workspace in its own
	// right. A path that fails to load gets an error row rather than
	// aborting the run, so it can't hide the other paths.
	var workspaces []*gomod.WorkFile
	var failed []Result
	if !v.opts.recursive {
		modPaths := make([]string, 0, len(paths))
		for _, path := range paths {
			goWork, err := v.findWorkspaceAt(path)
			switch {
			case err != nil:
				failed = append(failed, resultError(workspaceFileAt(path), err))
			case goWork != nil:
				workspaces = append(workspaces, goWork)
			default:
				modPaths = append(modPaths, path)
			}
		}
		paths = modPaths
	}

	modFiles, missingResults, err := v.findModFiles(paths)
	if err != nil {
		return nil, err
	}

	p := pool.NewWithResults[Result]().WithContext(ctx)
	for _, goWork := range workspaces {
		p.Go(func(ctx context.Context) (Result, error) {
			return v.processSource(ctx, goWork, filepath.Join(goWork.Dir, gomod.WorkFilename), goWork.WorkspaceConfig()), nil
		})
	}
	for _, modFile := range modFiles {
		p.Go(func(ctx context.Context) (Result, error) {
			goMod, err := gomod.Parse(modFile)
			if err != nil {
				return resultError(modFile, err), nil
			}
			return v.processSource(ctx, goMod, modFile, nil), nil
		})
	}

	processed, _ := p.Wait()
	results := append(append(failed, missingResults...), processed...)

	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})

	for _, r := range results {
		if r.Status.IsFailure() {
			return results, ErrVendorFailed
		}
	}

	return results, nil
}

func (v *Vendor) toResults(r Result) ([]Result, error) {
	if r.Status.IsFailure() {
		return []Result{r}, ErrVendorFailed
	}
	return []Result{r}, nil
}

// processSource implements the common drift detection and generation algorithm
// for both *gomod.File and *gomod.WorkFile sources. It always runs full
// resolution and compares the resulting manifest against the existing one —
// byte-for-byte equality is the drift signal. This catches all classes of
// change including package list updates, not just go.mod-level directives.
func (v *Vendor) processSource(ctx context.Context, src dependencySource, displayPath string, workspace *manifest.Workspace) Result {
	dir := filepath.Dir(displayPath)
	vendorPath := filepath.Join(dir, manifest.Filename)

	existingData, err := os.ReadFile(vendorPath)
	var existingMods map[string]manifest.Module

	if os.IsNotExist(err) {
		if v.opts.detectDrift {
			return resultMissing(displayPath)
		}
	} else if err != nil {
		return resultError(displayPath, err)
	} else {
		existing, err := manifest.Parse(existingData)
		if err != nil {
			return resultError(displayPath, err)
		}
		if existing.Schema != manifest.SchemaVersion && v.opts.detectDrift {
			return resultSchemaMismatch(displayPath, existing.Schema, manifest.SchemaVersion)
		}
		existingMods = existing.Mod
	}

	deps, rawTools, excludes, err := v.resolveSource(ctx, src, existingMods)
	if err != nil {
		return resultError(displayPath, err)
	}

	newData, depCount, err := v.generate(deps, rawTools, excludes, workspace)
	if err != nil {
		return resultError(displayPath, err)
	}

	unchanged := existingData != nil && bytes.Equal(newData, existingData)

	if v.opts.detectDrift {
		if unchanged {
			return resultOK(displayPath)
		}
		return resultDrift(displayPath)
	}

	if unchanged {
		return resultOK(displayPath)
	}

	if err := atomicWrite(vendorPath, newData); err != nil {
		return resultError(displayPath, err)
	}

	return resultGenerated(displayPath, depCount)
}

// resolveSource dispatches to the appropriate resolver based on the source
// type and returns the raw inputs needed to build a manifest.
func (v *Vendor) resolveSource(ctx context.Context, src dependencySource, existingMods map[string]manifest.Module) (deps []manifest.Module, rawTools []string, excludes map[string][]string, err error) {
	switch s := src.(type) {
	case *gomod.File:
		deps, err = v.resolver.ResolveModule(ctx, s, existingMods)
		rawTools = s.Tools
		if len(s.Excludes) > 0 {
			excludes = s.Excludes
		}
	case *gomod.WorkFile:
		var members []gomod.WorkspaceMember
		members, err = s.ParseMembers()
		if err != nil {
			return
		}
		deps, err = v.resolver.ResolveWorkspace(ctx, s, members, existingMods)
		if err != nil {
			return
		}
		merged := make(map[string][]string)
		for _, m := range members {
			rawTools = append(rawTools, m.Tools...)
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
		err = fmt.Errorf("unsupported dependency source: %T", src)
	}
	return
}

// generate builds and serialises a manifest from already-resolved dependency
// data. It has no knowledge of the source type.
func (v *Vendor) generate(deps []manifest.Module, rawTools []string, excludes map[string][]string, workspace *manifest.Workspace) ([]byte, int, error) {
	// Build a package→version lookup from resolved deps so each tool entry
	// records its own module version rather than the application version.
	var tool manifest.Tools
	if len(rawTools) > 0 {
		pkgToVersion := make(map[string]string)
		for _, dep := range deps {
			for _, pkg := range dep.Packages {
				pkgToVersion[pkg] = dep.Version
			}
		}
		slices.Sort(rawTools)
		rawTools = slices.Compact(rawTools)
		tool = make(manifest.Tools, len(rawTools))
		for _, pkg := range rawTools {
			tool[pkg] = manifest.Tool{Version: pkgToVersion[pkg]}
		}
	}

	m := manifest.New(deps, workspace, tool, excludes)

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		return nil, 0, err
	}

	return buf.Bytes(), len(m.Mod), nil
}

func (v *Vendor) findModFiles(paths []string) (modFiles []string, missing []Result, err error) {
	if v.opts.recursive {
		type scanResult struct {
			path  string
			files []string
		}

		p := pool.NewWithResults[scanResult]()
		for _, path := range paths {
			p.Go(func(_ context.Context) (scanResult, error) {
				scanner := NewFileTreeScanner(WithMaxDepth(v.opts.maxDepth))
				files, err := scanner.ScanFrom(path)
				return scanResult{path: path, files: files}, err
			})
		}

		results, err := p.Wait()
		if err != nil {
			return nil, nil, err
		}

		for _, scanned := range results {
			if len(scanned.files) == 0 {
				notFound := filepath.Join(scanned.path, gomod.Filename)
				if strings.HasPrefix(scanned.path, "./") {
					notFound = "./" + notFound
				}
				missing = append(missing, resultNotFound(notFound))
			} else {
				modFiles = append(modFiles, scanned.files...)
			}
		}

		return modFiles, missing, nil
	}

	for _, path := range paths {
		modPath := filepath.Join(path, gomod.Filename)
		if _, err := os.Stat(modPath); err != nil {
			missing = append(missing, resultNotFound(modPath))
		} else {
			modFiles = append(modFiles, modPath)
		}
	}

	return modFiles, missing, nil
}

func (v *Vendor) processWorkspaceMode(ctx context.Context) ([]Result, error) {
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
		result = resultMissing(filepath.Join(path, manifest.Filename))
	} else {
		manifestDir := filepath.Dir(manifestPath)
		goWork, err := v.findWorkspaceAt(manifestDir)
		if err != nil {
			result = resultError(manifestPath, err)
		} else if goWork == nil {
			result = resultError(manifestPath, fmt.Errorf("invalid workspace manifest"))
		} else {
			result = v.processSource(ctx, goWork, filepath.Join(manifestDir, gomod.WorkFilename), goWork.WorkspaceConfig())
		}
	}

	return v.toResults(result)
}

// workspaceFileAt names the file findWorkspaceAt read from path: go.work if
// present, otherwise govendor.toml.
func workspaceFileAt(path string) string {
	workPath := filepath.Join(path, gomod.WorkFilename)
	if _, err := os.Stat(workPath); err == nil {
		return workPath
	}
	return filepath.Join(path, manifest.Filename)
}

func (v *Vendor) findWorkspaceAt(path string) (*gomod.WorkFile, error) {
	workPath := filepath.Join(path, gomod.WorkFilename)
	if _, err := os.Stat(workPath); err == nil {
		goWork, err := gomod.ParseWork(workPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", workPath, err)
		}
		return goWork, nil
	}

	vendorPath := filepath.Join(path, manifest.Filename)
	data, err := os.ReadFile(vendorPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", vendorPath, err)
	}

	existing, err := manifest.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", vendorPath, err)
	}
	if existing.Workspace == nil {
		return nil, nil
	}

	goWork, err := gomod.NewWorkFileFromManifest(path, existing.Workspace)
	if err != nil {
		return nil, err
	}

	return goWork, nil
}

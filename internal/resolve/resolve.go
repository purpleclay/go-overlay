package resolve

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/purpleclay/conker/pool"
	"github.com/purpleclay/go-overlay/internal/mod"
	"github.com/purpleclay/go-overlay/internal/modulestxt"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

// Resolver resolves Go module dependencies via the Go toolchain. All external
// commands go through the Executor interface, making the resolver testable
// with injected output.
type Resolver struct {
	exec   Executor
	hasher Hasher
}

// New creates a Resolver with the given executor.
func New(exec Executor) *Resolver {
	return &Resolver{exec: exec, hasher: NARHasher{}}
}

// ResolveModule resolves all dependencies for a single Go module.
// existingMods is the parsed existing manifest's module map; pass nil for a cold run.
func (r *Resolver) ResolveModule(ctx context.Context, goMod *mod.GoModFile, existingMods map[string]mod.ModuleConfig) ([]mod.ModuleConfig, error) {
	vendored, err := r.vendorModules(ctx, goMod.Dir, []string{"GOWORK=off"}, "mod")
	if err != nil {
		return nil, err
	}

	downloads, err := r.downloadModules(ctx, goMod)
	if err != nil {
		return nil, err
	}

	pkgsByMod := make(map[string][]string, len(vendored))
	var remoteModules []modulestxt.Module
	for _, m := range vendored {
		pkgsByMod[m.Path] = m.Packages
		if m.Replace == nil || m.Replace.Local == "" {
			remoteModules = append(remoteModules, m)
		}
	}

	modules, err := r.resolveRemoteModules(ctx, remoteModules, downloads, existingMods)
	if err != nil {
		return nil, err
	}

	localModules, err := r.resolveLocalModules(ctx, goMod, pkgsByMod)
	if err != nil {
		return nil, err
	}

	modules = append(modules, localModules...)
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Path < modules[j].Path
	})

	return modules, nil
}

// ResolveWorkspace resolves dependencies across all modules in a Go workspace.
// A single go mod vendor pass from the workspace root replaces the per-platform
// go list fan-out. existingMods is the parsed existing manifest's module map;
// pass nil for a cold run.
func (r *Resolver) ResolveWorkspace(ctx context.Context, goWork *mod.GoWorkFile, existingMods map[string]mod.ModuleConfig) ([]mod.ModuleConfig, error) {
	members, err := goWork.ParseMembers()
	if err != nil {
		return nil, err
	}

	workspaceMembers := make(map[string]string, len(members))
	for _, m := range members {
		workspaceMembers[m.ModulePath] = m.Dir
	}

	// Single vendor pass from the workspace root with GOWORK active.
	vendored, err := r.vendorModules(ctx, goWork.Dir, nil, "work")
	if err != nil {
		return nil, err
	}

	// Download from the workspace root with GOWORK active so the Go toolchain
	// applies workspace-level MVS, producing one authoritative set of resolved
	// module versions rather than per-member independent resolutions.
	downloads, err := r.downloadWorkspaceModules(ctx, goWork)
	if err != nil {
		return nil, err
	}

	pkgsByMod := make(map[string][]string, len(vendored))
	var remoteModules []modulestxt.Module
	for _, m := range vendored {
		pkgsByMod[m.Path] = m.Packages
		if m.Replace == nil || m.Replace.Local == "" {
			remoteModules = append(remoteModules, m)
		}
	}

	// Parse each member go.mod once up front so both local replacement
	// passes can reuse the result without duplicate file I/O.
	memberGoMods := make(map[string]*mod.GoModFile, len(goWork.Modules))
	for _, modDir := range goWork.Modules {
		goModPath := filepath.Join(goWork.Dir, modDir, mod.GoModFilename)
		goMod, err := mod.ParseGoModFile(goModPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", goModPath, err)
		}
		memberGoMods[modDir] = goMod
	}

	remoteDeps, err := r.resolveRemoteModules(ctx, remoteModules, downloads, existingMods)
	if err != nil {
		return nil, err
	}

	// Resolve local replacements per member, preserving each member's base
	// directory for relative path resolution.
	allDeps := make(map[string]mod.ModuleConfig, len(remoteDeps))
	for _, dep := range remoteDeps {
		allDeps[dep.Path] = dep
	}

	for _, modDir := range goWork.Modules {
		localDeps, err := r.resolveLocalModules(ctx, memberGoMods[modDir], pkgsByMod)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve local modules for %s: %w", modDir, err)
		}
		for _, dep := range localDeps {
			// Workspace-level remote replacements take precedence — do not
			// overwrite an entry already resolved from go.work replace directives.
			if _, exists := allDeps[dep.Path]; !exists {
				allDeps[dep.Path] = dep
			}
		}
	}

	workspaceLocalDeps, err := r.resolveWorkspaceLocalModules(ctx, goWork, memberGoMods, downloads, pkgsByMod)
	if err != nil {
		return nil, err
	}
	for _, dep := range workspaceLocalDeps {
		allDeps[dep.Path] = dep
	}

	// Workspace members that are also required by other members appear in
	// downloads but not in modules.txt. Emit them as local source entries.
	downloadVersions := make(map[string]string, len(downloads))
	for _, dl := range downloads {
		downloadVersions[dl.Path] = dl.Version
	}
	for modulePath, localDir := range workspaceMembers {
		if version, isDep := downloadVersions[modulePath]; isDep {
			allDeps[modulePath] = mod.ModuleConfig{
				Path:    modulePath,
				Version: version,
				Local:   localDir,
			}
		}
	}

	modules := make([]mod.ModuleConfig, 0, len(allDeps))
	for _, m := range allDeps {
		modules = append(modules, m)
	}

	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Path < modules[j].Path
	})

	return modules, nil
}

// vendorModules runs go <verb> vendor -o <tmpdir> (verb is "mod" or "work"),
// parses the resulting modules.txt, and returns the ordered module list.
// The temp dir is always removed before this function returns.
func (r *Resolver) vendorModules(ctx context.Context, dir string, env []string, verb string) ([]modulestxt.Module, error) {
	tmpdir, err := os.MkdirTemp("", "govendor-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp vendor dir: %w", err)
	}
	defer os.RemoveAll(tmpdir)

	if _, err := r.exec.Run(ctx, []string{"go", verb, "vendor", "-o", tmpdir}, dir, env); err != nil {
		return nil, err
	}

	f, err := os.Open(filepath.Join(tmpdir, "modules.txt"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open vendor modules.txt: %w", err)
	}
	defer f.Close()

	return modulestxt.Parse(f)
}

func (r *Resolver) downloadModules(ctx context.Context, goMod *mod.GoModFile) ([]ModuleDownload, error) {
	args := []string{"go", "mod", "download", "-json"}
	env := []string{"GOWORK=off"}

	out, err := r.exec.Run(ctx, args, goMod.Dir, env)
	if err != nil {
		return nil, err
	}

	return ParseDownloadOutput(out)
}

// downloadWorkspaceModules runs go mod download from the workspace root with
// GOWORK active, letting the Go toolchain apply workspace-level MVS.
func (r *Resolver) downloadWorkspaceModules(ctx context.Context, goWork *mod.GoWorkFile) ([]ModuleDownload, error) {
	args := []string{"go", "mod", "download", "-json"}

	out, err := r.exec.Run(ctx, args, goWork.Dir, nil)
	if err != nil {
		return nil, err
	}

	return ParseDownloadOutput(out)
}

func (r *Resolver) resolveRemoteModules(ctx context.Context, modules []modulestxt.Module, downloads []ModuleDownload, existingMods map[string]mod.ModuleConfig) ([]mod.ModuleConfig, error) {
	if len(modules) == 0 {
		return nil, nil
	}

	dlByPath := make(map[string]ModuleDownload, len(downloads))
	for _, dl := range downloads {
		dlByPath[dl.Path] = dl
	}

	p := pool.NewWithResults[mod.ModuleConfig]().WithMaxGoroutines(8).WithContext(ctx)

	for _, m := range modules {
		p.Go(func(_ context.Context) (mod.ModuleConfig, error) {
			path := m.Path
			var replacedPath string
			var meta ModuleDownload
			var ok bool

			if m.Replace != nil && m.Replace.Path != "" {
				// Remote replacement: the download is keyed by the replacement path.
				replacedPath = m.Replace.Path
				meta, ok = dlByPath[m.Replace.Path]
			} else {
				meta, ok = dlByPath[path]
			}

			if !ok {
				return mod.ModuleConfig{}, fmt.Errorf("module %s not found in download output", path)
			}

			// Warm path: remote (path, version) pairs are immutable under the
			// checksum DB — reuse hash and go version from the existing manifest
			// when the (path, version, replacedPath) triple matches.
			if existingMods != nil {
				if entry, ok := existingMods[path]; ok &&
					entry.Version == meta.Version &&
					entry.Local == "" &&
					entry.Hash != "" &&
					entry.ReplacedPath == replacedPath {
					goVersion := entry.GoVersion
					if goVersion == "" {
						goVersion = m.GoVersion
					}
					return mod.ModuleConfig{
						Path:         path,
						Version:      meta.Version,
						Packages:     m.Packages,
						Hash:         entry.Hash,
						GoVersion:    goVersion,
						ReplacedPath: replacedPath,
						Implicit:     !m.Explicit,
					}, nil
				}
			}

			// Cold path: hash the module.
			hash, err := r.hasher.Hash(meta.Dir)
			if err != nil {
				return mod.ModuleConfig{}, fmt.Errorf("failed to hash downloaded module %s@%s: %w", meta.Path, meta.Version, err)
			}

			return mod.ModuleConfig{
				Path:         path,
				Version:      meta.Version,
				Packages:     m.Packages,
				Hash:         hash,
				GoVersion:    m.GoVersion,
				ReplacedPath: replacedPath,
				Implicit:     !m.Explicit,
			}, nil
		})
	}

	return p.Wait()
}

func (r *Resolver) resolveLocalModules(ctx context.Context, goMod *mod.GoModFile, pkgsByMod map[string][]string) ([]mod.ModuleConfig, error) {
	localRepls := goMod.LocalReplacements()
	if len(localRepls) == 0 {
		return nil, nil
	}

	requires := goMod.Requires
	p := pool.NewWithResults[mod.ModuleConfig]().WithMaxGoroutines(8).WithContext(ctx)

	for _, repl := range localRepls {
		p.Go(func(ctx context.Context) (mod.ModuleConfig, error) {
			localDir := repl.LocalPath
			if !filepath.IsAbs(localDir) {
				localDir = filepath.Join(goMod.Dir, localDir)
			}
			localDir, err := filepath.Abs(localDir)
			if err != nil {
				return mod.ModuleConfig{}, fmt.Errorf("failed to resolve local module path %s: %w", repl.LocalPath, err)
			}

			tracked, err := GitTrackedFiles(ctx, r.exec, localDir)
			if err != nil {
				return mod.ModuleConfig{}, fmt.Errorf("failed to list git tracked files for local module %s: %w", repl.LocalPath, err)
			}

			hash, err := NARHashGitTracked(localDir, tracked)
			if err != nil {
				return mod.ModuleConfig{}, fmt.Errorf("failed to hash local module %s: %w", repl.LocalPath, err)
			}

			var goVersion string
			localGoMod := filepath.Join(localDir, mod.GoModFilename)
			if modData, err := os.ReadFile(localGoMod); err == nil {
				if mf, err := modfile.Parse(localGoMod, modData, nil); err == nil && mf.Go != nil {
					goVersion = mf.Go.Version
				}
			}

			version := requires[repl.OldPath]
			if version == "" {
				version = "v0.0.0"
			}

			return mod.ModuleConfig{
				Path:      repl.OldPath,
				Version:   version,
				Packages:  pkgsByMod[repl.OldPath],
				Hash:      hash,
				GoVersion: goVersion,
				Local:     repl.LocalPath,
			}, nil
		})
	}

	return p.Wait()
}

// resolveWorkspaceLocalModules hashes and builds ModuleConfig entries for
// local replace directives declared at the workspace level in go.work. These
// are not visible to per-member go.mod parsing, so they must be resolved
// separately using the workspace root as the base for relative path resolution.
func (r *Resolver) resolveWorkspaceLocalModules(ctx context.Context, goWork *mod.GoWorkFile, memberGoMods map[string]*mod.GoModFile, downloads []ModuleDownload, pkgsByMod map[string][]string) ([]mod.ModuleConfig, error) {
	workspaceLocalRepls := goWork.LocalReplacements()
	if len(workspaceLocalRepls) == 0 {
		return nil, nil
	}

	// Prefer the workspace build list (downloadWorkspaceModules applies full
	// MVS across all members and transitive dependencies) for version selection.
	// Fall back to member requires — highest version wins — for any module not
	// present in the build list (e.g. a pure local-only replacement).
	downloadVersions := make(map[string]string, len(downloads))
	for _, dl := range downloads {
		downloadVersions[dl.Path] = dl.Version
	}

	allRequires := make(map[string]string)
	for _, modDir := range goWork.Modules {
		for path, version := range memberGoMods[modDir].Requires {
			if existing, exists := allRequires[path]; !exists || semver.Compare(version, existing) > 0 {
				allRequires[path] = version
			}
		}
	}

	p := pool.NewWithResults[mod.ModuleConfig]().WithMaxGoroutines(8).WithContext(ctx)

	for _, repl := range workspaceLocalRepls {
		p.Go(func(ctx context.Context) (mod.ModuleConfig, error) {
			localDir := repl.LocalPath
			if !filepath.IsAbs(localDir) {
				localDir = filepath.Join(goWork.Dir, localDir)
			}
			localDir, err := filepath.Abs(localDir)
			if err != nil {
				return mod.ModuleConfig{}, fmt.Errorf("failed to resolve workspace local module path %s: %w", repl.LocalPath, err)
			}

			tracked, err := GitTrackedFiles(ctx, r.exec, localDir)
			if err != nil {
				return mod.ModuleConfig{}, fmt.Errorf("failed to list git tracked files for workspace local module %s: %w", repl.LocalPath, err)
			}

			hash, err := NARHashGitTracked(localDir, tracked)
			if err != nil {
				return mod.ModuleConfig{}, fmt.Errorf("failed to hash workspace local module %s: %w", repl.LocalPath, err)
			}

			var goVersion string
			localGoMod := filepath.Join(localDir, mod.GoModFilename)
			if modData, err := os.ReadFile(localGoMod); err == nil {
				if mf, err := modfile.Parse(localGoMod, modData, nil); err == nil && mf.Go != nil {
					goVersion = mf.Go.Version
				}
			}

			version := downloadVersions[repl.OldPath]
			if version == "" {
				version = allRequires[repl.OldPath]
			}
			if version == "" {
				version = "v0.0.0"
			}

			return mod.ModuleConfig{
				Path:      repl.OldPath,
				Version:   version,
				Packages:  pkgsByMod[repl.OldPath],
				Hash:      hash,
				GoVersion: goVersion,
				Local:     repl.LocalPath,
			}, nil
		})
	}

	return p.Wait()
}

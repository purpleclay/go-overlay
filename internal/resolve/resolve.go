package resolve

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/purpleclay/conker/pool"
	"github.com/purpleclay/go-overlay/internal/mod"
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

// ValidatePlatforms checks that all given platform strings are supported by
// the current Go toolchain.
func (r *Resolver) ValidatePlatforms(ctx context.Context, platforms []string) error {
	if len(platforms) == 0 {
		return nil
	}

	out, err := r.exec.Run(ctx, []string{"go", "tool", "dist", "list"}, ".", nil)
	if err != nil {
		return fmt.Errorf("failed to get supported platforms: %w", err)
	}

	supported := make(map[string]bool)
	for line := range strings.SplitSeq(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			supported[line] = true
		}
	}

	var invalid []string
	for _, p := range platforms {
		if !supported[p] {
			invalid = append(invalid, p)
		}
	}

	if len(invalid) > 0 {
		return fmt.Errorf("unsupported platform(s): %s", strings.Join(invalid, ", "))
	}

	return nil
}

// ResolveModule resolves all dependencies for a single Go module.
// existingMods is the parsed existing manifest's module map; pass nil for a cold run.
func (r *Resolver) ResolveModule(ctx context.Context, goMod *mod.GoModFile, platforms []string, existingMods map[string]mod.ModuleConfig) ([]mod.ModuleConfig, error) {
	if platforms == nil {
		platforms = mod.DefaultPlatforms()
	}

	pkgsByMod, err := r.packagesByModule(ctx, goMod, platforms)
	if err != nil {
		return nil, err
	}

	downloads, err := r.downloadModules(ctx, goMod)
	if err != nil {
		return nil, err
	}

	modules, err := r.resolveRemoteModules(ctx, goMod.RemoteReplacements(), downloads, pkgsByMod, existingMods)
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
// It runs a single go mod download from the workspace root so Go's MVS applies
// across all members, then gathers per-member package attribution with GOWORK=off.
// existingMods is the parsed existing manifest's module map; pass nil for a cold run.
func (r *Resolver) ResolveWorkspace(ctx context.Context, goWork *mod.GoWorkFile, platforms []string, existingMods map[string]mod.ModuleConfig) ([]mod.ModuleConfig, error) {
	if platforms == nil {
		platforms = mod.DefaultPlatforms()
	}

	members, err := goWork.ParseMembers()
	if err != nil {
		return nil, err
	}

	workspaceMembers := make(map[string]string, len(members))
	for _, m := range members {
		workspaceMembers[m.ModulePath] = m.Dir
	}

	// Download from the workspace root with GOWORK active so the Go toolchain
	// applies workspace-level MVS, producing one authoritative set of resolved
	// module versions rather than per-member independent resolutions.
	downloads, err := r.downloadWorkspaceModules(ctx, goWork)
	if err != nil {
		return nil, err
	}

	// Parse each member go.mod once up front so both the packages and local
	// replacement passes can reuse the result without duplicate file I/O.
	memberGoMods := make(map[string]*mod.GoModFile, len(goWork.Modules))
	for _, modDir := range goWork.Modules {
		goModPath := filepath.Join(goWork.Dir, modDir, mod.GoModFilename)
		goMod, err := mod.ParseGoModFile(goModPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", goModPath, err)
		}
		memberGoMods[modDir] = goMod
	}

	// List packages for all workspace members in a single go list invocation per
	// platform from the workspace root, keeping GOWORK active so workspace-level
	// replace directives (including local replaces) are respected.
	pkgsByMod, err := r.packagesByWorkspace(ctx, goWork, memberGoMods, platforms)
	if err != nil {
		return nil, err
	}

	// Gather remote replacements from all members, workspace-level taking precedence.
	remoteRepls := make(map[string]mod.Replacement)
	for _, modDir := range goWork.Modules {
		maps.Copy(remoteRepls, memberGoMods[modDir].RemoteReplacements())
	}
	maps.Copy(remoteRepls, goWork.RemoteReplacements())

	remoteDeps, err := r.resolveRemoteModules(ctx, remoteRepls, downloads, pkgsByMod, existingMods)
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

	modules := make([]mod.ModuleConfig, 0, len(allDeps))
	for _, m := range allDeps {
		if localPath, isWorkspace := workspaceMembers[m.Path]; isWorkspace {
			m.Hash = ""
			m.Packages = nil
			m.Local = localPath
		}
		modules = append(modules, m)
	}

	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Path < modules[j].Path
	})

	return modules, nil
}

func (r *Resolver) packagesByModule(ctx context.Context, goMod *mod.GoModFile, platforms []string) (map[string][]string, error) {
	p := pool.NewWithResults[map[string][]string]().WithContext(ctx)

	seen := make(map[string]struct{}, len(platforms))
	for _, plat := range platforms {
		goos, goarch, ok := strings.Cut(plat, "/")
		if !ok || goos == "" || goarch == "" {
			return nil, fmt.Errorf("invalid platform %q: expected GOOS/GOARCH", plat)
		}
		if _, dup := seen[plat]; dup {
			continue
		}
		seen[plat] = struct{}{}
		p.Go(func(ctx context.Context) (map[string][]string, error) {
			return r.packagesByModuleForPlatform(ctx, goMod, goos, goarch)
		})
	}

	results, err := p.Wait()
	if err != nil {
		return nil, err
	}

	merged := make(map[string][]string)
	for _, result := range results {
		for m, pkgs := range result {
			merged[m] = append(merged[m], pkgs...)
		}
	}

	for modPath := range merged {
		sort.Strings(merged[modPath])
		merged[modPath] = slices.Compact(merged[modPath])
	}

	return merged, nil
}

func (r *Resolver) packagesByModuleForPlatform(ctx context.Context, goMod *mod.GoModFile, goos, goarch string) (map[string][]string, error) {
	listFmt := fmt.Sprintf(`{{if not .Standard}}{{if .Module}}{{if ne .Module.Path "%s"}}{{.Module.Path}}{{"\t"}}{{.ImportPath}}{{end}}{{end}}{{end}}`, goMod.ModulePath)

	args := []string{
		"go", "list", "-deps", "-test", "-f", listFmt, "./...",
	}

	// GOWORK=off ensures this module is processed independently, which is
	// essential for workspaces where each module's dependencies must be
	// resolved in isolation before being merged at the workspace level.
	env := []string{
		"GOWORK=off",
		"GOOS=" + goos,
		"GOARCH=" + goarch,
	}

	out, err := r.exec.Run(ctx, args, goMod.Dir, env)
	if err != nil {
		return nil, err
	}

	pkgsByMod := ParsePackagesByModule(out)

	// Include tool dependencies (Go 1.24+) so their packages appear in the
	// module-to-package mapping and are listed in modules.txt. A separate
	// invocation without -test avoids pulling in each tool's test-only
	// dependencies.
	if goMod.HasTools() {
		toolArgs := []string{
			"go", "list", "-deps", "-f", listFmt, "tool",
		}

		toolOut, err := r.exec.Run(ctx, toolArgs, goMod.Dir, env)
		if err != nil {
			return nil, err
		}

		for m, pkgs := range ParsePackagesByModule(toolOut) {
			pkgsByMod[m] = append(pkgsByMod[m], pkgs...)
		}
	}

	return pkgsByMod, nil
}

// packagesByWorkspace resolves package-to-module attribution for all workspace
// members in a single go list invocation per platform, run from the workspace
// root with GOWORK active. This ensures workspace-level replace directives
// (including local replaces) are respected, unlike per-member GOWORK=off listing.
func (r *Resolver) packagesByWorkspace(ctx context.Context, goWork *mod.GoWorkFile, memberGoMods map[string]*mod.GoModFile, platforms []string) (map[string][]string, error) {
	p := pool.NewWithResults[map[string][]string]().WithContext(ctx)

	seen := make(map[string]struct{}, len(platforms))
	for _, plat := range platforms {
		goos, goarch, ok := strings.Cut(plat, "/")
		if !ok || goos == "" || goarch == "" {
			return nil, fmt.Errorf("invalid platform %q: expected GOOS/GOARCH", plat)
		}
		if _, dup := seen[plat]; dup {
			continue
		}
		seen[plat] = struct{}{}
		p.Go(func(ctx context.Context) (map[string][]string, error) {
			return r.packagesByWorkspaceForPlatform(ctx, goWork, memberGoMods, goos, goarch)
		})
	}

	results, err := p.Wait()
	if err != nil {
		return nil, err
	}

	merged := make(map[string][]string)
	for _, result := range results {
		for m, pkgs := range result {
			merged[m] = append(merged[m], pkgs...)
		}
	}

	for modPath := range merged {
		sort.Strings(merged[modPath])
		merged[modPath] = slices.Compact(merged[modPath])
	}

	return merged, nil
}

func (r *Resolver) packagesByWorkspaceForPlatform(ctx context.Context, goWork *mod.GoWorkFile, memberGoMods map[string]*mod.GoModFile, goos, goarch string) (map[string][]string, error) {
	// Build import path patterns for every workspace member so a single go list
	// spans the full workspace, keeping GOWORK active so workspace-level replace
	// directives are respected.
	patterns := make([]string, 0, len(memberGoMods))
	for _, goMod := range memberGoMods {
		patterns = append(patterns, goMod.ModulePath+"/...")
	}
	sort.Strings(patterns)

	listFmt := `{{if not .Standard}}{{if .Module}}{{.Module.Path}}{{"\t"}}{{.ImportPath}}{{end}}{{end}}`
	args := append([]string{"go", "list", "-deps", "-test", "-f", listFmt}, patterns...)

	env := []string{
		"GOOS=" + goos,
		"GOARCH=" + goarch,
	}

	out, err := r.exec.Run(ctx, args, goWork.Dir, env)
	if err != nil {
		return nil, err
	}

	pkgsByMod := ParsePackagesByModule(out)

	// Tool packages are module-scoped and cannot be batched from the workspace
	// root, so they are listed per-member with GOWORK=off.
	toolEnv := []string{
		"GOWORK=off",
		"GOOS=" + goos,
		"GOARCH=" + goarch,
	}

	for _, goMod := range memberGoMods {
		if !goMod.HasTools() {
			continue
		}
		toolFmt := fmt.Sprintf(`{{if not .Standard}}{{if .Module}}{{if ne .Module.Path "%s"}}{{.Module.Path}}{{"\t"}}{{.ImportPath}}{{end}}{{end}}{{end}}`, goMod.ModulePath)
		toolOut, err := r.exec.Run(ctx, []string{"go", "list", "-deps", "-f", toolFmt, "tool"}, goMod.Dir, toolEnv)
		if err != nil {
			return nil, err
		}
		for m, pkgs := range ParsePackagesByModule(toolOut) {
			pkgsByMod[m] = append(pkgsByMod[m], pkgs...)
		}
	}

	return pkgsByMod, nil
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

// goVersionFromMod best-effort reads the go directive from a downloaded
// module's go.mod. Returns "" when the path is empty, unreadable, or unparsable.
func goVersionFromMod(goModPath string) string {
	if goModPath == "" {
		return ""
	}
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}
	mf, err := modfile.Parse(goModPath, data, nil)
	if err != nil || mf.Go == nil {
		return ""
	}
	return mf.Go.Version
}

func (r *Resolver) resolveRemoteModules(ctx context.Context, remoteReplacements map[string]mod.Replacement, downloads []ModuleDownload, pkgsByMod map[string][]string, existingMods map[string]mod.ModuleConfig) ([]mod.ModuleConfig, error) {
	p := pool.NewWithResults[mod.ModuleConfig]().WithMaxGoroutines(8).WithContext(ctx)

	for _, meta := range downloads {
		p.Go(func(_ context.Context) (mod.ModuleConfig, error) {
			// Determine the manifest key and replacement path before any I/O.
			path := meta.Path
			var replacedPath string
			if repl, ok := remoteReplacements[path]; ok {
				path = repl.OldPath
				replacedPath = meta.Path
			}

			// Remote (path, version) pairs are immutable under the checksum DB.
			// Reuse hash and go version from the existing manifest when the
			// (path, version, replacedPath) triple matches and the entry is remote.
			if existingMods != nil {
				if entry, ok := existingMods[path]; ok &&
					entry.Version == meta.Version &&
					entry.Local == "" &&
					entry.Hash != "" &&
					entry.ReplacedPath == replacedPath {
					goVersion := entry.GoVersion
					if goVersion == "" {
						goVersion = goVersionFromMod(meta.GoMod)
					}
					return mod.ModuleConfig{
						Path:         path,
						Version:      meta.Version,
						Packages:     pkgsByMod[path],
						Hash:         entry.Hash,
						GoVersion:    goVersion,
						ReplacedPath: replacedPath,
					}, nil
				}
			}

			hash, err := r.hasher.Hash(meta.Dir)
			if err != nil {
				return mod.ModuleConfig{}, fmt.Errorf("failed to hash downloaded module %s@%s: %w", meta.Path, meta.Version, err)
			}

			goVersion := goVersionFromMod(meta.GoMod)

			return mod.ModuleConfig{
				Path:         path,
				Version:      meta.Version,
				Packages:     pkgsByMod[path],
				Hash:         hash,
				GoVersion:    goVersion,
				ReplacedPath: replacedPath,
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

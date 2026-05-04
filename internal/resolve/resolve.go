package resolve

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/nix-community/go-nix/pkg/nar"
	"github.com/purpleclay/go-overlay/internal/mod"
	"github.com/sourcegraph/conc/pool"
	"golang.org/x/mod/modfile"
)

// Resolver resolves Go module dependencies via the Go toolchain. All external
// commands go through the Executor interface, making the resolver testable
// with injected output.
type Resolver struct {
	exec Executor
}

// New creates a Resolver with the given executor.
func New(exec Executor) *Resolver {
	return &Resolver{exec: exec}
}

// ValidatePlatforms checks that all given platform strings are supported by
// the current Go toolchain.
func (r *Resolver) ValidatePlatforms(platforms []string) error {
	if len(platforms) == 0 {
		return nil
	}

	out, err := r.exec.Run([]string{"go", "tool", "dist", "list"}, ".", nil)
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
func (r *Resolver) ResolveModule(goMod *mod.GoModFile, platforms []string) ([]mod.ModuleConfig, error) {
	if platforms == nil {
		platforms = mod.DefaultPlatforms()
	}

	pkgsByMod, err := r.packagesByModule(goMod, platforms)
	if err != nil {
		return nil, err
	}

	downloads, err := r.downloadModules(goMod)
	if err != nil {
		return nil, err
	}

	modules, err := r.resolveRemoteModules(goMod, downloads, pkgsByMod)
	if err != nil {
		return nil, err
	}

	localModules, err := r.resolveLocalModules(goMod, pkgsByMod)
	if err != nil {
		return nil, err
	}

	modules = append(modules, localModules...)
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Path < modules[j].Path
	})

	return modules, nil
}

// ResolveWorkspace resolves dependencies across all modules in a workspace,
// merging packages and post-processing workspace members (clearing hash and
// packages, since they're resolved from source rather than fetched).
func (r *Resolver) ResolveWorkspace(goWork *mod.GoWorkFile, platforms []string) ([]mod.ModuleConfig, error) {
	if platforms == nil {
		platforms = mod.DefaultPlatforms()
	}

	workspaceMembers, err := goWork.WorkspaceModulePaths()
	if err != nil {
		return nil, err
	}

	allDeps := make(map[string]mod.ModuleConfig)

	for _, modDir := range goWork.Modules() {
		goModPath := filepath.Join(goWork.Dir(), modDir, "go.mod")
		goMod, err := mod.ParseGoModFile(goModPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", goModPath, err)
		}

		deps, err := r.ResolveModule(goMod, platforms)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve dependencies for %s: %w", modDir, err)
		}

		for _, dep := range deps {
			if localPath, isWorkspace := workspaceMembers[dep.Path]; isWorkspace {
				dep.Hash = ""
				dep.Packages = nil
				if dep.Local != "" {
					dep.Local = localPath
				}
			}

			if existing, ok := allDeps[dep.Path]; ok {
				existing.Packages = MergePackages(existing.Packages, dep.Packages)
				allDeps[dep.Path] = existing
			} else {
				allDeps[dep.Path] = dep
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

func (r *Resolver) packagesByModule(goMod *mod.GoModFile, platforms []string) (map[string][]string, error) {
	p := pool.NewWithResults[map[string][]string]().WithErrors()

	seen := make(map[string]struct{}, len(platforms))
	for _, plat := range platforms {
		goos, goarch, ok := strings.Cut(plat, "/")
		if !ok {
			continue
		}
		if _, dup := seen[plat]; dup {
			continue
		}
		seen[plat] = struct{}{}
		p.Go(func() (map[string][]string, error) {
			return r.packagesByModuleForPlatform(goMod, goos, goarch)
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

func (r *Resolver) packagesByModuleForPlatform(goMod *mod.GoModFile, goos, goarch string) (map[string][]string, error) {
	listFmt := fmt.Sprintf(`{{if not .Standard}}{{if .Module}}{{if ne .Module.Path "%s"}}{{.Module.Path}}{{"\t"}}{{.ImportPath}}{{end}}{{end}}{{end}}`, goMod.ModulePath())

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

	out, err := r.exec.Run(args, goMod.Dir(), env)
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

		toolOut, err := r.exec.Run(toolArgs, goMod.Dir(), env)
		if err != nil {
			return nil, err
		}

		for m, pkgs := range ParsePackagesByModule(toolOut) {
			pkgsByMod[m] = append(pkgsByMod[m], pkgs...)
		}
	}

	return pkgsByMod, nil
}

func (r *Resolver) downloadModules(goMod *mod.GoModFile) ([]ModuleDownload, error) {
	args := []string{"go", "mod", "download", "-json"}
	env := []string{"GOWORK=off"}

	out, err := r.exec.Run(args, goMod.Dir(), env)
	if err != nil {
		return nil, err
	}

	return ParseDownloadOutput(out)
}

func (r *Resolver) resolveRemoteModules(goMod *mod.GoModFile, downloads []ModuleDownload, pkgsByMod map[string][]string) ([]mod.ModuleConfig, error) {
	remoteReplacements := goMod.RemoteReplacements()

	p := pool.NewWithResults[mod.ModuleConfig]().WithErrors().WithMaxGoroutines(8)

	for _, meta := range downloads {
		p.Go(func() (mod.ModuleConfig, error) {
			hash, err := NARHash(meta.Dir)
			if err != nil {
				return mod.ModuleConfig{}, fmt.Errorf("failed to hash downloaded module %s@%s: %w", meta.Path, meta.Version, err)
			}

			var goVersion string
			if meta.GoMod != "" {
				if modData, err := os.ReadFile(meta.GoMod); err == nil {
					if mf, err := modfile.Parse(meta.GoMod, modData, nil); err == nil && mf.Go != nil {
						goVersion = mf.Go.Version
					}
				}
			}

			path := meta.Path
			var replacedPath string
			if repl, ok := remoteReplacements[path]; ok {
				path = repl.OldPath
				replacedPath = meta.Path
			}

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

func (r *Resolver) resolveLocalModules(goMod *mod.GoModFile, pkgsByMod map[string][]string) ([]mod.ModuleConfig, error) {
	localRepls := goMod.LocalReplacements()
	if len(localRepls) == 0 {
		return nil, nil
	}

	requires := goMod.Requires()
	p := pool.NewWithResults[mod.ModuleConfig]().WithErrors().WithMaxGoroutines(8)

	for _, repl := range localRepls {
		p.Go(func() (mod.ModuleConfig, error) {
			localDir := repl.LocalPath
			if !filepath.IsAbs(localDir) {
				localDir = filepath.Join(goMod.Dir(), localDir)
			}
			localDir, err := filepath.Abs(localDir)
			if err != nil {
				return mod.ModuleConfig{}, fmt.Errorf("failed to resolve local module path %s: %w", repl.LocalPath, err)
			}

			tracked, err := GitTrackedFiles(r.exec, localDir)
			if err != nil {
				return mod.ModuleConfig{}, fmt.Errorf("failed to list git tracked files for local module %s: %w", repl.LocalPath, err)
			}

			hash, err := NARHashFiltered(localDir, func(path string, _ nar.NodeType) bool {
				if strings.ToLower(filepath.Base(path)) == ".ds_store" {
					return false
				}
				_, ok := tracked[path]
				return ok
			})
			if err != nil {
				return mod.ModuleConfig{}, fmt.Errorf("failed to hash local module %s: %w", repl.LocalPath, err)
			}

			var goVersion string
			localGoMod := filepath.Join(localDir, "go.mod")
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

package mod

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
)

type GoWorkFile struct {
	dir             string
	modules         []string
	hash            string
	workfile        *modfile.WorkFile
	workspaceConfig *WorkspaceConfig
}

func ParseGoWorkFile(path string) (*GoWorkFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.work: %w", err)
	}

	wf, err := modfile.ParseWork(path, content, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.work: %w", err)
	}

	var modules []string
	for _, use := range wf.Use {
		modules = append(modules, use.Path)
	}

	dir := filepath.Dir(path)
	hash, err := computeWorkspaceHash(dir, modules)
	if err != nil {
		return nil, err
	}

	return &GoWorkFile{
		dir:      dir,
		modules:  modules,
		hash:     hash,
		workfile: wf,
	}, nil
}

func NewGoWorkFileFromManifest(dir string, config *WorkspaceConfig) (*GoWorkFile, error) {
	modules := make([]string, len(config.Modules))
	for i, mod := range config.Modules {
		modules[i] = strings.TrimPrefix(mod, "./")
	}

	hash, err := computeWorkspaceHash(dir, modules)
	if err != nil {
		return nil, err
	}

	return &GoWorkFile{
		dir:             dir,
		modules:         modules,
		hash:            hash,
		workspaceConfig: config,
	}, nil
}

func computeWorkspaceHash(dir string, modules []string) (string, error) {
	h := sha256.New()

	sortedModules := slices.Clone(modules)
	slices.Sort(sortedModules)

	for _, mod := range sortedModules {
		modPath := filepath.Join(dir, mod, goModFile)
		content, err := os.ReadFile(modPath)
		if err != nil {
			return "", fmt.Errorf("failed to read %s: %w", modPath, err)
		}
		h.Write(content)
	}

	return "sha256-" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

func (w *GoWorkFile) Dir() string {
	return w.dir
}

func (w *GoWorkFile) Hash() string {
	return w.hash
}

func (w *GoWorkFile) Modules() []string {
	return w.modules
}

func (w *GoWorkFile) ModulePaths() []string {
	var paths []string
	for _, mod := range w.modules {
		paths = append(paths, filepath.Join(w.dir, mod))
	}
	return paths
}

func (w *GoWorkFile) Dependencies(extraPlatforms []string) ([]GoModule, error) {
	allDeps := make(map[string]GoModule)
	workspaceMembers := w.workspaceModulePaths()

	for _, modDir := range w.modules {
		goModPath := filepath.Join(w.dir, modDir, goModFile)
		goMod, err := ParseGoModFile(goModPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", goModPath, err)
		}

		deps, err := goMod.Dependencies(extraPlatforms)
		if err != nil {
			return nil, fmt.Errorf("failed to get dependencies for %s: %w", modDir, err)
		}

		for _, dep := range deps {
			// Post-process workspace members: clear hash and packages
			// They're resolved from source, not fetched
			if localPath, isWorkspace := workspaceMembers[dep.Path]; isWorkspace {
				dep.Hash = ""
				dep.Packages = nil

				// Convert local path to be relative to workspace root
				if dep.Local != "" {
					dep.Local = localPath
				}
			}

			// Dedup/merge dependencies
			if existing, ok := allDeps[dep.Path]; ok {
				existing.Packages = mergePackages(existing.Packages, dep.Packages)
				allDeps[dep.Path] = existing
			} else {
				allDeps[dep.Path] = dep
			}
		}
	}

	modules := make([]GoModule, 0, len(allDeps))
	for _, mod := range allDeps {
		modules = append(modules, mod)
	}
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Path < modules[j].Path
	})

	return modules, nil
}

func mergePackages(a, b []string) []string {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}

	seen := make(map[string]bool)
	for _, p := range a {
		seen[p] = true
	}
	for _, p := range b {
		seen[p] = true
	}

	result := make([]string, 0, len(seen))
	for p := range seen {
		result = append(result, p)
	}
	sort.Strings(result)
	return result
}

func (w *GoWorkFile) WorkspaceConfig() *WorkspaceConfig {
	if w.workspaceConfig != nil {
		return w.workspaceConfig
	}

	modules := make([]string, 0, len(w.modules))
	for _, mod := range w.modules {
		path := mod
		if !strings.HasPrefix(path, "./") {
			path = "./" + path
		}
		modules = append(modules, path)
	}
	slices.Sort(modules)

	config := &WorkspaceConfig{
		Modules: modules,
	}

	if w.workfile.Go != nil {
		config.Go = w.workfile.Go.Version
	}

	if w.workfile.Toolchain != nil {
		config.Toolchain = w.workfile.Toolchain.Name
	}

	return config
}

func (w *GoWorkFile) workspaceModulePaths() map[string]string {
	members := make(map[string]string)

	for _, mod := range w.modules {
		modFilePath := filepath.Join(w.dir, mod, goModFile)
		content, err := os.ReadFile(modFilePath)
		if err != nil {
			continue
		}

		mf, err := modfile.Parse(modFilePath, content, nil)
		if err != nil {
			continue
		}

		path := mod
		if !strings.HasPrefix(path, "./") {
			path = "./" + path
		}

		members[mf.Module.Mod.Path] = path
	}

	return members
}

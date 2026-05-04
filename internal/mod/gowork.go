package mod

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	if config == nil {
		return nil, fmt.Errorf("workspace config is required")
	}
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

// normalizeWorkspaceMemberPath prepends "./" to bare relative paths (e.g.
// "cli" → "./cli") while leaving absolute paths, ".", "./…" and "../…"
// unchanged.
func normalizeWorkspaceMemberPath(path string) string {
	if filepath.IsAbs(path) || path == "." || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
		return path
	}
	return "./" + path
}

func (w *GoWorkFile) WorkspaceConfig() *WorkspaceConfig {
	if w.workspaceConfig != nil {
		return w.workspaceConfig
	}

	modules := make([]string, 0, len(w.modules))
	for _, mod := range w.modules {
		modules = append(modules, normalizeWorkspaceMemberPath(mod))
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

	w.workspaceConfig = config
	return config
}

// WorkspaceModulePaths reads each workspace member's go.mod to map Go module
// paths to their relative directory paths. Returns an error if any member's
// go.mod is unreadable or unparsable.
func (w *GoWorkFile) WorkspaceModulePaths() (map[string]string, error) {
	members := make(map[string]string, len(w.modules))

	for _, mod := range w.modules {
		modFilePath := filepath.Join(w.dir, mod, goModFile)
		content, err := os.ReadFile(modFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read workspace member %s: %w", modFilePath, err)
		}

		mf, err := modfile.Parse(modFilePath, content, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to parse workspace member %s: %w", modFilePath, err)
		}

		if mf.Module == nil || mf.Module.Mod.Path == "" {
			return nil, fmt.Errorf("workspace member %s is missing a module directive", modFilePath)
		}

		members[mf.Module.Mod.Path] = normalizeWorkspaceMemberPath(mod)
	}

	return members, nil
}

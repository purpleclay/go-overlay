package mod

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
)

const GoWorkFilename = "go.work"

// WorkspaceMember holds the parsed metadata for a single workspace
// member's go.mod.
type WorkspaceMember struct {
	Dir          string
	ModulePath   string
	Requires     map[string]string
	Replacements map[string]Replacement
	Excludes     map[string][]string
}

// GoWorkFile is a parsed go.work file. All fields are extracted at
// parse time. No methods shell out to external processes.
type GoWorkFile struct {
	Dir       string
	GoVersion string
	Toolchain string
	Modules   []string
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

	modules := make([]string, len(wf.Use))
	for i, use := range wf.Use {
		modules[i] = use.Path
	}

	var goVersion string
	if wf.Go != nil {
		goVersion = wf.Go.Version
	}

	var toolchain string
	if wf.Toolchain != nil {
		toolchain = wf.Toolchain.Name
	}

	return &GoWorkFile{
		Dir:       filepath.Dir(path),
		GoVersion: goVersion,
		Toolchain: toolchain,
		Modules:   modules,
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

	return &GoWorkFile{
		Dir:       dir,
		GoVersion: config.Go,
		Toolchain: config.Toolchain,
		Modules:   modules,
	}, nil
}

func (w *GoWorkFile) ModulePaths() []string {
	paths := make([]string, len(w.Modules))
	for i, mod := range w.Modules {
		paths[i] = filepath.Join(w.Dir, mod)
	}
	return paths
}

func (w *GoWorkFile) WorkspaceConfig() *WorkspaceConfig {
	modules := make([]string, len(w.Modules))
	for i, mod := range w.Modules {
		modules[i] = normalizeWorkspaceMemberPath(mod)
	}
	slices.Sort(modules)

	return &WorkspaceConfig{
		Go:        w.GoVersion,
		Toolchain: w.Toolchain,
		Modules:   modules,
	}
}

// ParseMembers reads and parses each workspace member's go.mod in one
// pass, returning the module path, relative directory, and requires for
// each member.
func (w *GoWorkFile) ParseMembers() ([]WorkspaceMember, error) {
	members := make([]WorkspaceMember, 0, len(w.Modules))

	for _, mod := range w.Modules {
		modFilePath := filepath.Join(w.Dir, mod, GoModFilename)
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

		requires := make(map[string]string, len(mf.Require))
		for _, req := range mf.Require {
			requires[req.Mod.Path] = req.Mod.Version
		}

		replacements := parseReplacements(mf.Replace)
		excludes := parseExcludes(mf.Exclude)

		members = append(members, WorkspaceMember{
			Dir:          normalizeWorkspaceMemberPath(mod),
			ModulePath:   mf.Module.Mod.Path,
			Requires:     requires,
			Replacements: replacements,
			Excludes:     excludes,
		})
	}

	return members, nil
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

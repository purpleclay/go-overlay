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

// WorkspaceMember is a workspace member's parsed go.mod, plus its directory
// as listed in go.work.
type WorkspaceMember struct {
	*GoModFile
	// RelDir is the member directory relative to go.work, normalised to "./…".
	RelDir string
}

// GoWorkFile is a parsed go.work file. All fields are extracted at
// parse time. No methods shell out to external processes.
type GoWorkFile struct {
	Dir          string
	GoVersion    string
	Toolchain    string
	Modules      []string
	Replacements map[string]Replacement
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
		Dir:          filepath.Dir(path),
		GoVersion:    goVersion,
		Toolchain:    toolchain,
		Modules:      modules,
		Replacements: parseReplacements(wf.Replace),
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

// LocalReplacements returns only workspace-level local replace directives,
// sorted by original module path. These take precedence over member go.mod replaces.
func (w *GoWorkFile) LocalReplacements() []Replacement {
	var local []Replacement
	for _, repl := range w.Replacements {
		if repl.IsLocal {
			local = append(local, repl)
		}
	}
	slices.SortFunc(local, func(a, b Replacement) int {
		return strings.Compare(a.OldPath, b.OldPath)
	})
	return local
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

// ParseMembers reads and parses each workspace member's go.mod, in go.work
// order.
func (w *GoWorkFile) ParseMembers() ([]WorkspaceMember, error) {
	members := make([]WorkspaceMember, 0, len(w.Modules))

	for _, mod := range w.Modules {
		modFilePath := filepath.Join(w.Dir, mod, GoModFilename)
		goMod, err := ParseGoModFile(modFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse workspace member %s: %w", modFilePath, err)
		}

		if goMod.ModulePath == "" {
			return nil, fmt.Errorf("workspace member %s is missing a module directive", modFilePath)
		}

		members = append(members, WorkspaceMember{
			GoModFile: goMod,
			RelDir:    normalizeWorkspaceMemberPath(mod),
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

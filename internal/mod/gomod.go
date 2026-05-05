package mod

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
)

const GoModFilename = "go.mod"

// GoModFile is a parsed go.mod file. All fields are extracted at parse
// time. No methods shell out to external processes.
type GoModFile struct {
	Dir          string
	ModulePath   string
	GoVersion    string
	Requires     map[string]string
	Tools        []string
	Replacements map[string]Replacement
}

func ParseGoModFile(path string) (*GoModFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.mod: %w", err)
	}

	mf, err := modfile.Parse(path, content, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.mod: %w", err)
	}

	if mf.Module == nil {
		return nil, fmt.Errorf("go.mod is missing a module directive: %s", path)
	}

	var goVersion string
	if mf.Go != nil {
		goVersion = mf.Go.Version
	}

	requires := make(map[string]string, len(mf.Require))
	for _, req := range mf.Require {
		requires[req.Mod.Path] = req.Mod.Version
	}

	tools := make([]string, len(mf.Tool))
	for i, t := range mf.Tool {
		tools[i] = t.Path
	}

	replacements := make(map[string]Replacement, len(mf.Replace))
	for _, repl := range mf.Replace {
		isLocal := strings.HasPrefix(repl.New.Path, ".") || filepath.IsAbs(repl.New.Path)
		replacements[repl.Old.Path] = Replacement{
			OldPath:   repl.Old.Path,
			NewPath:   repl.New.Path,
			IsLocal:   isLocal,
			LocalPath: repl.New.Path,
		}
	}

	return &GoModFile{
		Dir:          filepath.Dir(path),
		ModulePath:   mf.Module.Mod.Path,
		GoVersion:    goVersion,
		Requires:     requires,
		Tools:        tools,
		Replacements: replacements,
	}, nil
}

func (f *GoModFile) HasDependencies() bool {
	return len(f.Requires) > 0
}

func (f *GoModFile) HasTools() bool {
	return len(f.Tools) > 0
}

type Replacement struct {
	OldPath   string
	NewPath   string
	IsLocal   bool
	LocalPath string
}

// LocalReplacements returns only local replace directives (those pointing to
// relative or absolute paths), sorted by original module path.
func (f *GoModFile) LocalReplacements() []Replacement {
	var local []Replacement
	for _, repl := range f.Replacements {
		if repl.IsLocal {
			local = append(local, repl)
		}
	}
	slices.SortFunc(local, func(a, b Replacement) int {
		return strings.Compare(a.OldPath, b.OldPath)
	})
	return local
}

// RemoteReplacements returns a map keyed by the replacement target path (the
// right side of replace A => B). This keying matches go mod download output,
// which uses the replacement target path.
func (f *GoModFile) RemoteReplacements() map[string]Replacement {
	remote := make(map[string]Replacement)
	for _, repl := range f.Replacements {
		if !repl.IsLocal {
			remote[repl.NewPath] = repl
		}
	}
	return remote
}

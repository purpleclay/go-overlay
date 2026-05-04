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

const goModFile = "go.mod"

// GoModFile is a parsed go.mod file. It holds the raw content, parsed AST,
// and a content hash for drift detection. No methods on this type shell out
// to external processes.
type GoModFile struct {
	dir     string
	content []byte
	modfile *modfile.File
	hash    string
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

	h := sha256.Sum256(content)
	hash := "sha256-" + base64.StdEncoding.EncodeToString(h[:])

	return &GoModFile{
		dir:     filepath.Dir(path),
		content: content,
		modfile: mf,
		hash:    hash,
	}, nil
}

func (f *GoModFile) Dir() string {
	return f.dir
}

func (f *GoModFile) Hash() string {
	return f.hash
}

func (f *GoModFile) ModulePath() string {
	return f.modfile.Module.Mod.Path
}

func (f *GoModFile) HasDependencies() bool {
	return len(f.modfile.Require) > 0
}

func (f *GoModFile) HasTools() bool {
	return len(f.modfile.Tool) > 0
}

func (f *GoModFile) GoVersion() string {
	if f.modfile.Go != nil {
		return f.modfile.Go.Version
	}
	return ""
}

// Requires returns a map of module path to version for all required
// dependencies.
func (f *GoModFile) Requires() map[string]string {
	requires := make(map[string]string, len(f.modfile.Require))
	for _, req := range f.modfile.Require {
		requires[req.Mod.Path] = req.Mod.Version
	}
	return requires
}

type Replacement struct {
	OldPath   string
	NewPath   string
	IsLocal   bool
	LocalPath string
}

// Replacements returns all replace directives keyed by the original module
// path (the left side of the replace directive).
func (f *GoModFile) Replacements() map[string]Replacement {
	replacements := make(map[string]Replacement, len(f.modfile.Replace))
	for _, repl := range f.modfile.Replace {
		isLocal := strings.HasPrefix(repl.New.Path, ".") || filepath.IsAbs(repl.New.Path)
		replacements[repl.Old.Path] = Replacement{
			OldPath:   repl.Old.Path,
			NewPath:   repl.New.Path,
			IsLocal:   isLocal,
			LocalPath: repl.New.Path,
		}
	}
	return replacements
}

// LocalReplacements returns only local replace directives (those pointing to
// relative or absolute paths), sorted by original module path.
func (f *GoModFile) LocalReplacements() []Replacement {
	var local []Replacement
	for _, repl := range f.Replacements() {
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
// right side of replace A => B). This keying matches `go mod download`
// output, which uses the replacement target path.
func (f *GoModFile) RemoteReplacements() map[string]Replacement {
	remote := make(map[string]Replacement)
	for _, repl := range f.Replacements() {
		if !repl.IsLocal {
			remote[repl.NewPath] = repl
		}
	}
	return remote
}

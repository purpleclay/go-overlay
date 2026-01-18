package mod

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charlievieth/fastwalk"
)

var skipDirs = map[string]struct{}{
	"__pycache__":  {},
	".cache":       {},
	".devenv":      {},
	".direnv":      {},
	".git":         {},
	".gradle":      {},
	".idea":        {},
	".mvn":         {},
	".terraform":   {},
	".venv":        {},
	".vscode":      {},
	".zed":         {},
	"bin":          {},
	"build":        {},
	"dist":         {},
	"node_modules": {},
	"obj":          {},
	"out":          {},
	"packages":     {},
	"result":       {},
	"target":       {},
	"testdata":     {},
	"vendor":       {},
	"venv":         {},
	"zig-cache":    {},
	"zig-out":      {},
}

type scanOptions struct {
	maxDepth int
}

type ScanOption func(*scanOptions)

func WithMaxDepth(depth int) ScanOption {
	return func(opts *scanOptions) {
		opts.maxDepth = depth
	}
}

type FileTreeScanner struct {
	opts scanOptions
}

func NewFileTreeScanner(opts ...ScanOption) *FileTreeScanner {
	s := &FileTreeScanner{}
	for _, opt := range opts {
		opt(&s.opts)
	}
	return s
}

func (s *FileTreeScanner) ScanFrom(dir string) ([]string, error) {
	var paths []string
	var mu sync.Mutex

	conf := fastwalk.Config{
		Follow:   false,
		MaxDepth: s.opts.maxDepth,
	}

	err := fastwalk.Walk(&conf, dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if _, skip := skipDirs[d.Name()]; skip {
				return fastwalk.SkipDir
			}
			return nil
		}

		if d.Name() == goModFile {
			mu.Lock()
			paths = append(paths, strings.TrimPrefix(path, "./"))
			mu.Unlock()
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return paths, nil
}

// FindWorkspaceManifest walks up the directory tree from a submodule path
// looking for a govendor.toml file. The maximum depth is derived from the
// path itself (number of path components), preventing traversal beyond
// the expected project root.
//
// For example, given "theme/go.mod", the path is cleaned to "theme" which
// has 1 component, allowing traversal up 1 level to find the workspace manifest.
func FindWorkspaceManifest(submodulePath string) (string, error) {
	// Strip the filename if present (e.g., "theme/go.mod" -> "theme")
	if base := filepath.Base(submodulePath); base == goModFile || base == goWorkFile || base == vendorFile {
		submodulePath = filepath.Dir(submodulePath)
	}

	current, err := filepath.Abs(submodulePath)
	if err != nil {
		return "", err
	}

	// Count path components to determine max depth
	// "theme" -> 1, "a/b/c" -> 3, "." -> 0
	cleaned := filepath.Clean(submodulePath)
	maxDepth := 0
	if cleaned != "." {
		maxDepth = strings.Count(cleaned, string(filepath.Separator)) + 1
	}

	depth := 0
	for {
		candidate := filepath.Join(current, vendorFile)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", nil
		}

		depth++
		if depth > maxDepth {
			return "", nil
		}

		current = parent
	}
}

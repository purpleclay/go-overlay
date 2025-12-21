package mod

import (
	"io/fs"
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

type FileScanner struct {
	opts scanOptions
}

func NewFileScanner(opts ...ScanOption) *FileScanner {
	s := &FileScanner{}
	for _, opt := range opts {
		opt(&s.opts)
	}
	return s
}

func (s *FileScanner) ScanFrom(dir string) ([]string, error) {
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
			paths = append(paths, path)
			mu.Unlock()
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return paths, nil
}

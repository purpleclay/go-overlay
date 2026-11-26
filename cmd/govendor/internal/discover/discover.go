package discover

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/purpleclay/conker/pool"
	"github.com/purpleclay/go-overlay/internal/gomod"
	"github.com/purpleclay/go-overlay/internal/manifest"
)

// Kind says whether a Target is a single module or a workspace.
type Kind int

const (
	Module Kind = iota
	Workspace
)

// Target is one go.mod or go.work for govendor to process. Its go.mod or
// go.work and any govendor.toml are read up front, so nothing downstream
// reads them again.
type Target struct {
	Kind Kind
	// Path is the go.mod or go.work path. When Err is set, it may instead name
	// the file that failed to load, such as govendor.toml.
	Path string
	// Mod is set for a Module, Work for a Workspace.
	Mod  *gomod.File
	Work *gomod.WorkFile
	// Existing is the raw govendor.toml beside Path, or nil when there is none.
	Existing []byte
	// Err records a target that could not be loaded. It is reported as that
	// target's result rather than stopping the run, so it can't hide others.
	Err error
}

// ErrNoManifest marks a --workspace search that found no govendor.toml.
var ErrNoManifest = errors.New("govendor.toml not found")

type options struct {
	recursive bool
	maxDepth  int
	workspace bool
}

type Option func(*options)

// WithRecursive scans each path for go.mod files, down to maxDepth levels
// (0 = unlimited). Workspaces are not detected in this mode.
func WithRecursive(maxDepth int) Option {
	return func(opts *options) {
		opts.recursive = true
		opts.maxDepth = maxDepth
	}
}

// WithWorkspace walks up from the first path to the govendor.toml of the
// workspace it belongs to.
func WithWorkspace() Option {
	return func(opts *options) {
		opts.workspace = true
	}
}

// Targets works out what to process from paths, which may name directories
// or go.mod, go.work and govendor.toml files. No paths means ".". The error
// is reserved for failures that stop discovery as a whole.
func Targets(paths []string, opts ...Option) ([]Target, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	dirs := make([]string, 0, len(paths))
	for _, path := range paths {
		if base := filepath.Base(path); base == gomod.Filename || base == gomod.WorkFilename || base == manifest.Filename {
			path = filepath.Dir(path)
		}
		dirs = append(dirs, path)
	}
	if len(dirs) == 0 {
		dirs = []string{"."}
	}

	switch {
	case o.workspace:
		t, err := enclosingWorkspace(dirs[0])
		if err != nil {
			return nil, err
		}
		return []Target{t}, nil
	case o.recursive:
		return scan(dirs, o.maxDepth)
	}

	// Without --recursive, every path is checked for a workspace in its own
	// right.
	targets := make([]Target, 0, len(dirs))
	for _, dir := range dirs {
		ws, existing, err := workspaceAt(dir)
		switch {
		case err != nil:
			targets = append(targets, Target{Path: workspaceFileAt(dir), Err: err})
		case ws != nil:
			targets = append(targets, *ws)
		default:
			modPath := filepath.Join(dir, gomod.Filename)
			if _, err := os.Stat(modPath); err != nil {
				targets = append(targets, notFound(modPath))
				continue
			}
			targets = append(targets, moduleAt(modPath, existing))
		}
	}
	return targets, nil
}

func scan(dirs []string, maxDepth int) ([]Target, error) {
	type scanResult struct {
		dir   string
		files []string
	}

	p := pool.NewWithResults[scanResult]()
	for _, dir := range dirs {
		p.Go(func(_ context.Context) (scanResult, error) {
			files, err := NewFileTreeScanner(WithMaxDepth(maxDepth)).ScanFrom(dir)
			return scanResult{dir: dir, files: files}, err
		})
	}

	scanned, err := p.Wait()
	if err != nil {
		return nil, err
	}

	var targets []Target
	for _, s := range scanned {
		if len(s.files) == 0 {
			modPath := filepath.Join(s.dir, gomod.Filename)
			if strings.HasPrefix(s.dir, "./") {
				modPath = "./" + modPath
			}
			targets = append(targets, notFound(modPath))
			continue
		}
		for _, modPath := range s.files {
			existing, err := readExisting(filepath.Dir(modPath))
			if err != nil {
				targets = append(targets, Target{Path: modPath, Err: err})
				continue
			}
			targets = append(targets, moduleAt(modPath, existing))
		}
	}
	return targets, nil
}

func enclosingWorkspace(dir string) (Target, error) {
	manifestPath, err := FindWorkspaceManifest(dir)
	if err != nil {
		return Target{}, err
	}
	if manifestPath == "" {
		return Target{Kind: Workspace, Path: filepath.Join(dir, manifest.Filename), Err: ErrNoManifest}, nil
	}

	ws, _, err := workspaceAt(filepath.Dir(manifestPath))
	switch {
	case err != nil:
		return Target{Kind: Workspace, Path: manifestPath, Err: err}, nil
	case ws == nil:
		return Target{Kind: Workspace, Path: manifestPath, Err: errors.New("invalid workspace manifest")}, nil
	}
	return *ws, nil
}

func moduleAt(modPath string, existing []byte) Target {
	goMod, err := gomod.Parse(modPath)
	if err != nil {
		return Target{Kind: Module, Path: modPath, Err: err}
	}
	return Target{Kind: Module, Path: modPath, Mod: goMod, Existing: existing}
}

func notFound(modPath string) Target {
	return Target{Kind: Module, Path: modPath, Err: fmt.Errorf("%s does not exist, check path", gomod.Filename)}
}

// workspaceAt loads the workspace in dir, from go.work if present, otherwise
// from the [workspace] table of govendor.toml. It returns a nil target when
// dir holds no workspace, along with any govendor.toml it read, so a module
// in dir does not read it again.
func workspaceAt(dir string) (*Target, []byte, error) {
	workPath := filepath.Join(dir, gomod.WorkFilename)
	if _, err := os.Stat(workPath); err == nil {
		goWork, err := gomod.ParseWork(workPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse %s: %w", workPath, err)
		}
		existing, err := readExisting(dir)
		if err != nil {
			return &Target{Kind: Workspace, Path: workPath, Err: err}, nil, nil
		}
		return &Target{Kind: Workspace, Path: workPath, Work: goWork, Existing: existing}, existing, nil
	}

	vendorPath := filepath.Join(dir, manifest.Filename)
	data, err := os.ReadFile(vendorPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("failed to read %s: %w", vendorPath, err)
	}

	existing, err := manifest.Parse(data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse %s: %w", vendorPath, err)
	}
	if existing.Workspace == nil {
		return nil, data, nil
	}

	goWork, err := gomod.NewWorkFileFromManifest(dir, existing.Workspace)
	if err != nil {
		return nil, nil, err
	}
	return &Target{Kind: Workspace, Path: workPath, Work: goWork, Existing: data}, data, nil
}

// workspaceFileAt names the file workspaceAt read from dir: go.work if
// present, otherwise govendor.toml.
func workspaceFileAt(dir string) string {
	workPath := filepath.Join(dir, gomod.WorkFilename)
	if _, err := os.Stat(workPath); err == nil {
		return workPath
	}
	return filepath.Join(dir, manifest.Filename)
}

// readExisting returns the govendor.toml in dir, or nil when there is none.
func readExisting(dir string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(dir, manifest.Filename))
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}

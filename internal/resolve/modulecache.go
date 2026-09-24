package resolve

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/mod/module"
)

// moduleCache computes a downloaded module's on-disk directory directly,
// matching the Go toolchain's own module cache layout
// (cmd/go/internal/modfetch.DownloadDir), instead of asking
// `go mod download` to report it.
//
// GOMODCACHE is looked up once, lazily, on first use — not read from the
// environment directly, since an unset GOMODCACHE still resolves to a real
// default (GOPATH/pkg/mod) that only `go env` knows how to compute.
type moduleCache struct {
	exec    Executor
	workDir string // working directory `go env` is run from

	once sync.Once
	root string
	err  error
}

// newModuleCache returns a moduleCache that runs `go env GOMODCACHE` from
// workDir the first time its directory is needed.
func newModuleCache(exec Executor, workDir string) *moduleCache {
	return &moduleCache{exec: exec, workDir: workDir}
}

func (c *moduleCache) resolveRoot(ctx context.Context) (string, error) {
	c.once.Do(func() {
		out, err := c.exec.Run(ctx, Command{Args: []string{"go", "env", "GOMODCACHE"}, Dir: c.workDir})
		if err != nil {
			c.err = fmt.Errorf("failed to determine GOMODCACHE: %w", err)
			return
		}
		c.root = strings.TrimSpace(out)
	})
	return c.root, c.err
}

// dir returns the module cache directory for path@version. The escaping
// matches what the Go toolchain itself uses for the same directory: an
// uppercase letter is replaced with "!" followed by its lowercase form, so
// e.g. github.com/BurntSushi/toml becomes github.com/!burnt!sushi/toml on
// disk.
func (c *moduleCache) dir(ctx context.Context, path, version string) (string, error) {
	root, err := c.resolveRoot(ctx)
	if err != nil {
		return "", err
	}

	escapedPath, err := module.EscapePath(path)
	if err != nil {
		return "", fmt.Errorf("failed to escape module path %s: %w", path, err)
	}
	escapedVersion, err := module.EscapeVersion(version)
	if err != nil {
		return "", fmt.Errorf("failed to escape module version %s: %w", version, err)
	}

	return filepath.Join(root, escapedPath+"@"+escapedVersion), nil
}

// downloadOne fetches exactly path@version. Unlike a bare or graph-wide `go
// mod download`, an explicitly-versioned single-module argument doesn't
// trigger MVS re-resolution, so it never touches go.sum or go.work.sum —
// confirmed empirically against a real cold module cache — which is what
// this package exists to avoid reintroducing.
func (c *moduleCache) downloadOne(ctx context.Context, path, version string) error {
	_, err := c.exec.Run(ctx, Command{
		Args: []string{"go", "mod", "download", path + "@" + version},
		Dir:  c.workDir,
	})
	return err
}

// cachedModuleDir returns a dirResolver that computes each module's cache
// directory directly and confirms it's already present, without invoking
// `go mod download` at all — the common case is a single `stat`, no
// subprocess, no network.
//
// A directory can legitimately be missing even though `go mod vendor`
// listed the module: an explicit direct dependency with no packages
// actually imported only needs its go.mod read for graph resolution, never
// its full source extracted. When that happens, downloadOne fetches
// exactly that one module and the stat is retried.
func cachedModuleDir(cache *moduleCache) dirResolver {
	return func(ctx context.Context, key contentKey) (string, error) {
		dir, err := cache.dir(ctx, key.path, key.version)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(dir); err == nil {
			return dir, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to check module cache directory for %s@%s: %w", key.path, key.version, err)
		}

		if err := cache.downloadOne(ctx, key.path, key.version); err != nil {
			return "", fmt.Errorf("module %s@%s not found in local module cache: %w", key.path, key.version, err)
		}
		if _, err := os.Stat(dir); err != nil {
			return "", fmt.Errorf("module %s@%s still missing from local module cache after download: %w", key.path, key.version, err)
		}
		return dir, nil
	}
}

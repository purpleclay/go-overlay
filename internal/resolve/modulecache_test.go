package resolve

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/module"
)

type callCountingExecutor struct {
	Executor
	mu    sync.Mutex
	calls int
}

func (e *callCountingExecutor) Run(ctx context.Context, c Command) (string, error) {
	e.mu.Lock()
	e.calls++
	e.mu.Unlock()
	return e.Executor.Run(ctx, c)
}

func TestModuleCacheDirEscapesUppercaseLetters(t *testing.T) {
	exec := &fakeExecutor{responses: map[string]string{"go env GOMODCACHE": "/root/go/pkg/mod\n"}}
	c := newModuleCache(exec, "/repo")

	dir, err := c.dir(context.Background(), "github.com/BurntSushi/toml", "v1.6.0")
	require.NoError(t, err)
	assert.Equal(t, "/root/go/pkg/mod/github.com/!burnt!sushi/toml@v1.6.0", dir)
}

func TestModuleCacheDirLeavesLowercasePathsUnchanged(t *testing.T) {
	exec := &fakeExecutor{responses: map[string]string{"go env GOMODCACHE": "/root/go/pkg/mod\n"}}
	c := newModuleCache(exec, "/repo")

	dir, err := c.dir(context.Background(), "golang.org/x/mod", "v0.41.0")
	require.NoError(t, err)
	assert.Equal(t, "/root/go/pkg/mod/golang.org/x/mod@v0.41.0", dir)
}

func TestModuleCacheCallsGoEnvOnlyOnce(t *testing.T) {
	exec := &callCountingExecutor{Executor: &fakeExecutor{responses: map[string]string{"go env GOMODCACHE": "/root/go/pkg/mod\n"}}}
	c := newModuleCache(exec, "/repo")

	_, err := c.dir(context.Background(), "github.com/spf13/cobra", "v1.10.2")
	require.NoError(t, err)
	_, err = c.dir(context.Background(), "golang.org/x/mod", "v0.41.0")
	require.NoError(t, err)
	_, err = c.dir(context.Background(), "github.com/spf13/cobra", "v1.10.2")
	require.NoError(t, err)

	exec.mu.Lock()
	defer exec.mu.Unlock()
	assert.Equal(t, 1, exec.calls, "GOMODCACHE must be looked up once regardless of how many modules ask for a directory")
}

func TestModuleCacheDirPropagatesGoEnvError(t *testing.T) {
	exec := &fakeExecutor{} // no "go env GOMODCACHE" response configured
	c := newModuleCache(exec, "/repo")

	_, err := c.dir(context.Background(), "example.com/foo", "v1.0.0")
	assert.Error(t, err)
}

func TestModuleCacheDirRejectsAnInvalidModulePath(t *testing.T) {
	exec := &fakeExecutor{responses: map[string]string{"go env GOMODCACHE": "/root/go/pkg/mod\n"}}
	c := newModuleCache(exec, "/repo")

	_, err := c.dir(context.Background(), "not a valid path", "v1.0.0")
	assert.Error(t, err)
}

func TestCachedModuleDirReturnsTheDirectoryWhenAlreadyPresent(t *testing.T) {
	gomodcache := t.TempDir()
	writeModuleCacheDir(t, gomodcache, "example.com/foo", "v1.0.0")

	exec := &fakeExecutor{responses: map[string]string{"go env GOMODCACHE": gomodcache + "\n"}}
	resolver := cachedModuleDir(newModuleCache(exec, "/repo"))

	dir, err := resolver(context.Background(), contentKey{path: "example.com/foo", version: "v1.0.0"})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(gomodcache, "example.com/foo@v1.0.0"), dir)
}

// downloadExtractsExecutor simulates the one side effect cachedModuleDir's
// fallback actually depends on: a real `go mod download <path>@<version>`
// extracting the module into GOMODCACHE. Confirmed empirically against the
// real Go toolchain that this exact single-argument form does so without
// touching go.sum or go.work.sum.
type downloadExtractsExecutor struct {
	gomodcache string
}

func (e *downloadExtractsExecutor) Run(_ context.Context, c Command) (string, error) {
	args := c.Args
	if len(args) >= 2 && args[0] == "go" && args[1] == "env" {
		return e.gomodcache + "\n", nil
	}
	if len(args) == 4 && args[0] == "go" && args[1] == "mod" && args[2] == "download" {
		path, version, ok := strings.Cut(args[3], "@")
		if !ok {
			return "", fmt.Errorf("malformed module argument: %s", args[3])
		}
		escapedPath, err := module.EscapePath(path)
		if err != nil {
			return "", err
		}
		escapedVersion, err := module.EscapeVersion(version)
		if err != nil {
			return "", err
		}
		dir := filepath.Join(e.gomodcache, escapedPath+"@"+escapedVersion)
		return "", os.MkdirAll(dir, 0o755)
	}
	return "", fmt.Errorf("unexpected command: %v", args)
}

func TestCachedModuleDirFallsBackToATargetedDownloadOnACacheMiss(t *testing.T) {
	gomodcache := t.TempDir()
	exec := &downloadExtractsExecutor{gomodcache: gomodcache}
	resolver := cachedModuleDir(newModuleCache(exec, "/repo"))

	// Nothing pre-populates gomodcache: the module isn't there yet, matching
	// an explicit direct dependency go mod vendor never had to extract
	// (nothing imports any of its packages).
	dir, err := resolver(context.Background(), contentKey{path: "github.com/pmezard/go-difflib", version: "v1.0.0"})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(gomodcache, "github.com/pmezard/go-difflib@v1.0.0"), dir)
	_, statErr := os.Stat(dir)
	assert.NoError(t, statErr, "expected the fallback download to have created the directory")
}

func TestCachedModuleDirReturnsAnErrorWhenTheFallbackDownloadFails(t *testing.T) {
	// No "go mod" response configured: the fallback download fails with
	// fakeExecutor's "unexpected command".
	exec := &fakeExecutor{responses: map[string]string{"go env GOMODCACHE": t.TempDir() + "\n"}}
	resolver := cachedModuleDir(newModuleCache(exec, "/repo"))

	_, err := resolver(context.Background(), contentKey{path: "example.com/missing", version: "v1.0.0"})
	assert.ErrorContains(t, err, "example.com/missing@v1.0.0 not found in local module cache")
	assert.ErrorContains(t, err, "unexpected command: go mod download example.com/missing@v1.0.0")
}

func TestCachedModuleDirReturnsAnErrorWhenStillMissingAfterASuccessfulDownload(t *testing.T) {
	// "go mod" matches any go mod subcommand fakeExecutor doesn't otherwise
	// special-case, including "go mod download ..." — standing in for a
	// download that reports success without actually leaving the directory
	// behind.
	exec := &fakeExecutor{responses: map[string]string{
		"go env GOMODCACHE": t.TempDir() + "\n",
		"go mod":            "",
	}}
	resolver := cachedModuleDir(newModuleCache(exec, "/repo"))

	_, err := resolver(context.Background(), contentKey{path: "example.com/foo", version: "v1.0.0"})
	assert.ErrorContains(t, err, "still missing")
}

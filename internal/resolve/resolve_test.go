package resolve

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/purpleclay/conker/pool"
	"github.com/purpleclay/go-overlay/internal/mod"
	"github.com/purpleclay/go-overlay/internal/modulestxt"
	"github.com/purpleclay/go-overlay/internal/progress"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/module"
)

// countingHasher counts Hash calls and returns a fixed or per-dir hash.
// Safe for concurrent use by the goroutine pool in resolveRemoteModules.
type countingHasher struct {
	count atomic.Int64
	// returned for every call; defaults to "sha256-"+dir
	hash string
	// when set, returned instead of a hash on every call
	err error
}

// HashGitTracked ignores the tracked-file set: these tests assert on how
// often hashing happens and what it returns, not on the NAR filter itself,
// which NARHashGitTracked's own tests cover.
func (h *countingHasher) HashGitTracked(dir string, _ map[string]struct{}) (string, error) {
	return h.Hash(dir)
}

func (h *countingHasher) Hash(dir string) (string, error) {
	h.count.Add(1)
	if h.err != nil {
		return "", h.err
	}
	if h.hash != "" {
		return h.hash, nil
	}
	return "sha256-" + dir, nil
}

// fakeExecutor returns canned responses keyed by either the full command
// string or the first two args (e.g. "go mod", "git ls-files").
// Special handling for "go mod vendor": writes the response string as
// modules.txt to the directory given by the -o flag.
type fakeExecutor struct {
	responses map[string]string
	// stderrNoise, keyed the same way as responses' vendor-command key
	// ("go mod vendor" / "go work vendor"), holds extra lines to stream via
	// onStderr before the modules.txt content — standing in for the
	// "go: downloading ..."/"warning: ..." lines a real `go mod vendor -v`
	// prints during the load phase, before any "# path version" header.
	stderrNoise map[string][]string
}

func (f *fakeExecutor) Run(_ context.Context, c Command) (string, error) {
	args := c.Args
	full := strings.Join(args, " ")

	// A vendor command with a stderr sink replays the canned response line
	// by line first — any configured stderrNoise, standing in for the
	// load-phase lines a real `go mod vendor -v` prints before the first
	// module header — then falls through to write modules.txt, exactly as a
	// real vendor pass leaves the file on disk once the process exits.
	if c.OnStderr != nil && len(args) >= 3 && args[0] == "go" && args[2] == "vendor" {
		key := args[0] + " " + args[1] + " " + args[2]
		if content, ok := f.responses[key]; ok {
			for _, line := range f.stderrNoise[key] {
				c.OnStderr(line)
			}
			for line := range strings.SplitSeq(strings.TrimRight(content, "\n"), "\n") {
				c.OnStderr(line)
			}
		}
	}

	if out, ok := f.responses[full]; ok {
		return out, nil
	}

	// "go mod vendor -o <dir>" / "go work vendor -o <dir>": write the
	// response as modules.txt to the directory given by the -o flag.
	if len(args) >= 3 && args[0] == "go" && args[2] == "vendor" {
		key := args[0] + " " + args[1] + " " + args[2]
		content, ok := f.responses[key]
		if !ok {
			return "", fmt.Errorf("unexpected command: %s", full)
		}
		for i, arg := range args {
			if arg == "-o" && i+1 < len(args) {
				err := os.WriteFile(filepath.Join(args[i+1], "modules.txt"), []byte(content), 0o644)
				return "", err
			}
		}
	}

	if len(args) >= 2 {
		key := args[0] + " " + args[1]
		if out, ok := f.responses[key]; ok {
			return out, nil
		}
	}

	return "", fmt.Errorf("unexpected command: %s", full)
}

// blockingExecutor records the args it was called with, then blocks until
// the context is cancelled and returns ctx.Err(), standing in for a real
// `go` subprocess that OSExecutor.Run has sent SIGINT to and is waiting on.
type blockingExecutor struct {
	args chan []string
}

func (b blockingExecutor) Run(ctx context.Context, c Command) (string, error) {
	b.args <- c.Args
	<-ctx.Done()
	return "", ctx.Err()
}

func TestSplitVendoredKeepsOnlyTheSelectedVersionOfAWorkspaceMVSConflict(t *testing.T) {
	// Shape confirmed against a real `go work vendor -v` run on a fixture
	// where one workspace member requires v1.1.0 and another v1.1.1: both
	// stream as separate modules.txt entries, but only the selected v1.1.1
	// has packages attached.
	vendored := []modulestxt.Module{
		{Path: "github.com/davecgh/go-spew", Version: "v1.1.0", Explicit: true},
		{Path: "github.com/davecgh/go-spew", Version: "v1.1.1", Explicit: true, Packages: []string{"github.com/davecgh/go-spew/spew"}},
	}

	pkgsByMod, remote := splitVendored(vendored)

	require.Len(t, remote, 1, "the superseded v1.1.0 entry must not be treated as needing its own hash")
	assert.Equal(t, "v1.1.1", remote[0].Version)
	assert.Equal(t, []string{"github.com/davecgh/go-spew/spew"}, pkgsByMod["github.com/davecgh/go-spew"])
}

func TestWorkspaceVersionsFindsAVersionVendoredExposesButNoMemberDirectlyRequires(t *testing.T) {
	// Mirrors a real repro: a workspace member requires only "mid", an
	// unpruned (pre-module-graph-pruning) intermediate dependency that
	// itself requires "leaf" — the target of a go.work-level local
	// replacement. "leaf" never appears in any member's own go.mod, but the
	// vendor pass's own per-member output still reports its true,
	// pruning-aware resolved version. Confirmed against the real Go
	// toolchain with a deliberately old (go 1.16) intermediate dependency.
	goWork := &mod.GoWorkFile{Modules: []string{"cli"}}
	memberGoMods := map[string]*mod.GoModFile{
		"cli": {Requires: map[string]string{"example.com/mid": "v0.0.0"}},
	}
	vendored := []modulestxt.Module{
		{Path: "example.com/mid", Version: "v0.0.0"},
		{Path: "example.com/leaf", Version: "v1.0.0", Replace: &modulestxt.Replace{Local: "./local-leaf"}},
	}

	versions := workspaceVersions(goWork, memberGoMods, vendored)

	assert.Equal(t, "v1.0.0", versions["example.com/leaf"])
}

func TestVendorModulesRemovesTempDirOnCancellation(t *testing.T) {
	exec := blockingExecutor{args: make(chan []string, 1)}
	r := &Resolver{exec: exec, hasher: &countingHasher{}, reporter: progress.NopReporter{}}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := r.vendorModules(ctx, "test", t.TempDir(), nil, "mod", nil)
	require.ErrorIs(t, err, context.Canceled)

	// The temp dir vendorModules created is the last arg of "go mod vendor
	// -o <tmpdir>" — assert on that specific path rather than scanning the
	// shared os.TempDir(), which could contain an unrelated govendor-* entry
	// from another process and fail this test for reasons unconnected to it.
	args := <-exec.args
	tmpdir := args[len(args)-1]
	_, statErr := os.Stat(tmpdir)
	assert.True(t, os.IsNotExist(statErr), "expected temp dir %s to have been removed", tmpdir)
}

// vendorFailsWhileDownloadBlocksExecutor streams one module header from the
// vendor command and then fails it, while a `go mod download` for that
// module — the cache-miss fallback — blocks until its context is cancelled
// (or the test ends), standing in for a stalled network fetch.
type vendorFailsWhileDownloadBlocksExecutor struct {
	gomodcache string
	release    chan struct{}
}

func (e *vendorFailsWhileDownloadBlocksExecutor) Run(ctx context.Context, c Command) (string, error) {
	switch {
	case len(c.Args) >= 3 && c.Args[1] == "env":
		return e.gomodcache + "\n", nil
	case len(c.Args) >= 3 && c.Args[2] == "download":
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-e.release:
			return "", errors.New("download released by test cleanup")
		}
	case len(c.Args) >= 3 && c.Args[2] == "vendor":
		c.OnStderr("# example.com/foo v1.0.0")
		c.OnStderr("## explicit")
		c.OnStderr("example.com/foo")
		return "", errors.New("go: vendor failed")
	}
	return "", fmt.Errorf("unexpected command: %v", c.Args)
}

func TestVendorModulesReturnsPromptlyWhenVendorFailsWhileADownloadIsInFlight(t *testing.T) {
	exec := &vendorFailsWhileDownloadBlocksExecutor{gomodcache: t.TempDir(), release: make(chan struct{})}
	t.Cleanup(func() { close(exec.release) })

	r := New(exec)
	scheduler := newHashScheduler(context.Background(), r.hashPool, r.hasher, r.reporter, "test", nil, cachedModuleDir(newModuleCache(exec, "/repo")))

	errc := make(chan error, 1)
	go func() {
		_, err := r.vendorModules(context.Background(), "test", t.TempDir(), nil, "mod", scheduler)
		errc <- err
	}()

	select {
	case err := <-errc:
		assert.ErrorContains(t, err, "vendor failed")
	case <-time.After(2 * time.Second):
		t.Fatal("vendorModules withheld the vendor error while a cache-miss download was still in flight")
	}
}

func TestVendorModulesReportsExpectedEventSequence(t *testing.T) {
	exec := &fakeExecutor{
		responses: map[string]string{
			"go mod vendor": `# github.com/fatih/color v1.18.0
## explicit; go 1.25.0
github.com/fatih/color
# github.com/mattn/go-colorable v0.1.13
## explicit; go 1.25.0
github.com/mattn/go-colorable`,
		},
		stderrNoise: map[string][]string{
			"go mod vendor": {
				"go: downloading github.com/fatih/color v1.18.0",
				"go: downloading github.com/mattn/go-colorable v0.1.13",
			},
		},
	}

	reporter := &recordingReporter{}
	r := New(exec, WithReporter(reporter))

	modules, err := r.vendorModules(context.Background(), "test", t.TempDir(), nil, "mod", nil)
	require.NoError(t, err)
	require.Len(t, modules, 2)

	// vendorModules reports only what it observes on the stream. Started
	// and Finished belong to the whole resolve and are reported by
	// ResolveModule/ResolveWorkspace — see TestResolveModuleBracketsEvents.
	require.Equal(t, []string{
		"progress.Downloading",
		"progress.Downloading",
		"progress.PhaseChanged",
		"progress.Vendored",
		"progress.Vendored",
	}, reporter.kinds())

	phase := reporter.events[2].(progress.PhaseChanged)
	assert.Equal(t, progress.Vendor, phase.Phase)

	first := reporter.events[3].(progress.Vendored)
	assert.Equal(t, "github.com/fatih/color", first.Path)
	second := reporter.events[4].(progress.Vendored)
	assert.Equal(t, "github.com/mattn/go-colorable", second.Path)
}

func TestResolveModuleReportsFinishedWithErrorOnFailure(t *testing.T) {
	dir := t.TempDir()
	goModPath := writeTestFile(t, dir, "go.mod", "module example.com/app\n\ngo 1.25.4\n")
	goMod, err := mod.ParseGoModFile(goModPath)
	require.NoError(t, err)

	// no "go mod vendor" entry — fakeExecutor.Run returns "unexpected
	// command" for it, standing in for a real toolchain failure.
	exec := &fakeExecutor{responses: map[string]string{}}

	reporter := &recordingReporter{}
	r := New(exec, WithReporter(reporter))

	_, err = r.ResolveModule(context.Background(), goMod, nil)
	require.Error(t, err)

	require.Equal(t, []string{"progress.Started", "progress.Finished"}, reporter.kinds())
	finished := reporter.events[1].(progress.Finished)
	assert.Error(t, finished.Err)
	assert.Equal(t, progress.Manifest(filepath.Join(dir, "go.mod")), finished.Source())
}

func TestResolveModuleBracketsEvents(t *testing.T) {
	dir := t.TempDir()
	goModPath := writeTestFile(t, dir, "go.mod", `
module example.com/app

go 1.25.4

require github.com/fatih/color v1.18.0
`)
	goMod, err := mod.ParseGoModFile(goModPath)
	require.NoError(t, err)

	gomodcache := t.TempDir()
	writeModuleCacheDir(t, gomodcache, "github.com/fatih/color", "v1.18.0")

	exec := &fakeExecutor{
		responses: map[string]string{
			"go mod vendor": `# github.com/fatih/color v1.18.0
## explicit; go 1.25.0
github.com/fatih/color`,
			"go env GOMODCACHE": gomodcache + "\n",
		},
	}

	hasher := &countingHasher{hash: "sha256-test"}
	reporter := &recordingReporter{}
	r := New(exec, WithReporter(reporter), WithHasher(hasher))

	_, err = r.ResolveModule(context.Background(), goMod, nil)
	require.NoError(t, err)

	require.Equal(t, []string{
		"progress.Started",
		"progress.PhaseChanged", // Vendor
		"progress.Vendored",
		"progress.HashStarted",
		"progress.Hashed",
		"progress.Finished",
	}, reporter.kinds(), "Finished must be reported only after download and hashing complete, not from inside vendorModules")

	hashed := reporter.events[4].(progress.Hashed)
	assert.Equal(t, "github.com/fatih/color", hashed.Path)
	assert.False(t, hashed.Reused, "a cold run hashes rather than reusing")

	assert.Equal(t, int64(1), hasher.count.Load(), "hashing must have already happened by the time Finished is reported")
}

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// writeModuleCacheDir creates an empty directory at path@version's location
// under gomodcache, escaped exactly as the real module cache would — so
// cachedModuleDir's stat check finds it, matching what a real, already-warm
// module cache looks like on disk.
func writeModuleCacheDir(t *testing.T, gomodcache, path, version string) string {
	t.Helper()
	escapedPath, err := module.EscapePath(path)
	require.NoError(t, err)
	escapedVersion, err := module.EscapeVersion(version)
	require.NoError(t, err)
	dir := filepath.Join(gomodcache, escapedPath+"@"+escapedVersion)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	return dir
}

func TestResolveModule(t *testing.T) {
	dir := t.TempDir()
	goModPath := writeTestFile(t, dir, "go.mod", `
module github.com/purpleclay/example/app

go 1.25.4

require github.com/fatih/color v1.18.0

require (
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
)
`)

	goMod, err := mod.ParseGoModFile(goModPath)
	require.NoError(t, err)

	gomodcache := t.TempDir()
	writeModuleCacheDir(t, gomodcache, "github.com/fatih/color", "v1.18.0")
	writeModuleCacheDir(t, gomodcache, "github.com/mattn/go-colorable", "v0.1.13")
	writeModuleCacheDir(t, gomodcache, "github.com/mattn/go-isatty", "v0.0.20")

	exec := &fakeExecutor{
		responses: map[string]string{
			"go mod vendor": `# github.com/fatih/color v1.18.0
## explicit; go 1.25.0
github.com/fatih/color
# github.com/mattn/go-colorable v0.1.13
## explicit; go 1.25.0
github.com/mattn/go-colorable
# github.com/mattn/go-isatty v0.0.20
## explicit; go 1.25.0
github.com/mattn/go-isatty`,
			"go env GOMODCACHE": gomodcache + "\n",
		},
	}

	r := New(exec)
	deps, err := r.ResolveModule(context.Background(), goMod, nil)
	require.NoError(t, err)
	require.Len(t, deps, 3)

	// Results must be sorted by module path
	assert.Equal(t, "github.com/fatih/color", deps[0].Path)
	assert.Equal(t, "v1.18.0", deps[0].Version)
	assert.Equal(t, "1.25.0", deps[0].GoVersion) // from modules.txt annotation
	assert.NotEmpty(t, deps[0].Hash)
	assert.Equal(t, []string{"github.com/fatih/color"}, deps[0].Packages)

	assert.Equal(t, "github.com/mattn/go-colorable", deps[1].Path)
	assert.Equal(t, "v0.1.13", deps[1].Version)

	assert.Equal(t, "github.com/mattn/go-isatty", deps[2].Path)
	assert.Equal(t, "v0.0.20", deps[2].Version)
}

func TestResolveModuleWithLocalReplacement(t *testing.T) {
	dir := t.TempDir()
	// Create the local module with real files so NARHashFiltered can walk it.
	writeTestFile(t, dir, "localmod/go.mod", "module example.com/localmod\n\ngo 1.25.4\n")
	writeTestFile(t, dir, "localmod/lib.go", "package localmod\n")

	goModPath := writeTestFile(t, dir, "go.mod", `
module example.com/app

go 1.25.4

require example.com/localmod v0.0.0

replace example.com/localmod => ./localmod
`)

	goMod, err := mod.ParseGoModFile(goModPath)
	require.NoError(t, err)

	exec := &fakeExecutor{
		responses: map[string]string{
			"go mod vendor": `# example.com/localmod => ./localmod
## explicit; go 1.25.4
example.com/localmod`,
			"go mod":       "", // no remote downloads
			"git ls-files": "go.mod\nlib.go",
		},
	}

	r := New(exec)
	deps, err := r.ResolveModule(context.Background(), goMod, nil)
	require.NoError(t, err)
	require.Len(t, deps, 1)

	assert.Equal(t, "example.com/localmod", deps[0].Path)
	assert.Equal(t, "v0.0.0", deps[0].Version)
	assert.Equal(t, "./localmod", deps[0].Local)
	assert.Equal(t, "1.25.4", deps[0].GoVersion)
	assert.Equal(t, []string{"example.com/localmod"}, deps[0].Packages)
	assert.NotEmpty(t, deps[0].Hash)
	assert.Empty(t, deps[0].ReplacedPath)
}

func TestSharedHashPoolSupportsConcurrentResolveLocalModulesCalls(t *testing.T) {
	dirA := t.TempDir()
	writeTestFile(t, dirA, "localmod/go.mod", "module example.com/a\n\ngo 1.25.4\n")
	writeTestFile(t, dirA, "localmod/lib.go", "package a\n")

	dirB := t.TempDir()
	writeTestFile(t, dirB, "localmod/go.mod", "module example.com/b\n\ngo 1.25.4\n")
	writeTestFile(t, dirB, "localmod/lib.go", "package b\n")

	exec := &fakeExecutor{responses: map[string]string{"git ls-files": "go.mod\nlib.go"}}
	r := New(exec)

	localsA := []localModule{{repl: mod.Replacement{OldPath: "example.com/a", LocalPath: "./localmod"}, baseDir: dirA}}
	localsB := []localModule{{repl: mod.Replacement{OldPath: "example.com/b", LocalPath: "./localmod"}, baseDir: dirB}}

	var depsA, depsB []mod.ModuleConfig
	var errA, errB error

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		depsA, errA = r.resolveLocalModules(context.Background(), localsA, nil)
	}()
	go func() {
		defer wg.Done()
		depsB, errB = r.resolveLocalModules(context.Background(), localsB, nil)
	}()
	wg.Wait()

	require.NoError(t, errA)
	require.NoError(t, errB)
	require.Len(t, depsA, 1, "call A must see only its own result, not tasks the shared pool ran for call B")
	require.Len(t, depsB, 1, "call B must see only its own result, not tasks the shared pool ran for call A")
	assert.Equal(t, "example.com/a", depsA[0].Path)
	assert.Equal(t, "example.com/b", depsB[0].Path)
}

func TestResolveModuleWithRemoteReplacement(t *testing.T) {
	dir := t.TempDir()
	goModPath := writeTestFile(t, dir, "go.mod", `
module github.com/purpleclay/example/app

go 1.25.4

require gopkg.in/ini.v1 v1.67.0

replace gopkg.in/ini.v1 => github.com/go-ini/ini v1.67.0
`)

	goMod, err := mod.ParseGoModFile(goModPath)
	require.NoError(t, err)

	gomodcache := t.TempDir()
	// The hash scheduler resolves the replacement target's cache directory
	// (github.com/go-ini/ini), not the original path. The resolver must
	// still map the result back to the original (gopkg.in/ini.v1) via the
	// Replace field in modules.txt.
	writeModuleCacheDir(t, gomodcache, "github.com/go-ini/ini", "v1.67.0")

	exec := &fakeExecutor{
		responses: map[string]string{
			"go mod vendor": `# gopkg.in/ini.v1 v1.67.0 => github.com/go-ini/ini v1.67.0
## explicit; go 1.14
gopkg.in/ini.v1`,
			"go env GOMODCACHE": gomodcache + "\n",
		},
	}

	r := New(exec)
	deps, err := r.ResolveModule(context.Background(), goMod, nil)
	require.NoError(t, err)
	require.Len(t, deps, 1)

	assert.Equal(t, "gopkg.in/ini.v1", deps[0].Path)
	assert.Equal(t, "v1.67.0", deps[0].Version)
	assert.Equal(t, "github.com/go-ini/ini", deps[0].ReplacedPath)
	assert.Equal(t, []string{"gopkg.in/ini.v1"}, deps[0].Packages)
	assert.NotEmpty(t, deps[0].Hash)
}

func TestResolveWorkspace(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "go.work", `
go 1.25.4

use (
	./cli
	./core
)
`)
	writeTestFile(t, dir, "cli/go.mod", `
module github.com/purpleclay/example/cli

go 1.25.4

require (
	github.com/fatih/color v1.18.0
	github.com/mattn/go-colorable v0.1.13 // indirect
)
`)
	writeTestFile(t, dir, "core/go.mod", `
module github.com/purpleclay/example/core

go 1.25.4

require (
	github.com/fatih/color v1.18.0
	github.com/mattn/go-isatty v0.0.20 // indirect
)
`)

	goWork, err := mod.ParseGoWorkFile(filepath.Join(dir, "go.work"))
	require.NoError(t, err)

	gomodcache := t.TempDir()
	writeModuleCacheDir(t, gomodcache, "github.com/fatih/color", "v1.18.0")
	writeModuleCacheDir(t, gomodcache, "github.com/mattn/go-colorable", "v0.1.13")
	writeModuleCacheDir(t, gomodcache, "github.com/mattn/go-isatty", "v0.0.20")

	// Both workspace modules share github.com/fatih/color. The resolver must
	// deduplicate it, merging packages from both modules into a single entry.
	exec := &fakeExecutor{
		responses: map[string]string{
			"go work vendor": `## workspace
# github.com/fatih/color v1.18.0
## explicit; go 1.25.0
github.com/fatih/color
# github.com/mattn/go-colorable v0.1.13
## explicit; go 1.25.0
github.com/mattn/go-colorable
# github.com/mattn/go-isatty v0.0.20
## explicit; go 1.25.0
github.com/mattn/go-isatty`,
			"go env GOMODCACHE": gomodcache + "\n",
		},
	}

	r := New(exec)
	deps, err := r.ResolveWorkspace(context.Background(), goWork, nil)
	require.NoError(t, err)
	require.Len(t, deps, 3)

	assert.Equal(t, "github.com/fatih/color", deps[0].Path)
	assert.Equal(t, "github.com/mattn/go-colorable", deps[1].Path)
	assert.Equal(t, "github.com/mattn/go-isatty", deps[2].Path)
}

func TestResolveWorkspacePostProcessesMembers(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "go.work", `
go 1.25.4

use (
	./cli
	./core
)
`)
	writeTestFile(t, dir, "cli/go.mod", `
module github.com/purpleclay/example/cli

go 1.25.4

require github.com/purpleclay/example/core v1.0.0
`)
	writeTestFile(t, dir, "core/go.mod", `
module github.com/purpleclay/example/core

go 1.25.4
`)

	goWork, err := mod.ParseGoWorkFile(filepath.Join(dir, "go.work"))
	require.NoError(t, err)

	// cli depends on core, which is also a workspace member. core does not
	// appear in modules.txt (workspace members are never vendored) — its
	// version comes directly from cli's own go.mod require line instead. The
	// resolver must emit it as a local source entry with empty hash and
	// packages.
	exec := &fakeExecutor{
		responses: map[string]string{
			"go work vendor": `## workspace`,
		},
	}

	r := New(exec)
	deps, err := r.ResolveWorkspace(context.Background(), goWork, nil)
	require.NoError(t, err)
	require.Len(t, deps, 1)

	assert.Equal(t, "github.com/purpleclay/example/core", deps[0].Path)
	assert.Empty(t, deps[0].Hash)
	assert.Empty(t, deps[0].Packages)
}

func dirResolverFromDownloads(downloads []ModuleDownload) dirResolver {
	byPath := make(map[string]string, len(downloads))
	for _, dl := range downloads {
		byPath[dl.Path] = dl.Dir
	}
	return func(_ context.Context, key contentKey) (string, error) {
		dir, ok := byPath[key.path]
		if !ok {
			return "", fmt.Errorf("module %s not found in download output", key.path)
		}
		return dir, nil
	}
}

func TestResolveRemoteModulesHashReuse(t *testing.T) {
	fooDownload := ModuleDownload{Path: "example.com/foo", Version: "v1.2.3", Dir: "/cache/foo"}
	barDownload := ModuleDownload{Path: "example.com/bar", Version: "v2.0.0", Dir: "/cache/bar"}

	fooExisting := mod.ModuleConfig{
		Path:      "example.com/foo",
		Version:   "v1.2.3",
		Hash:      "sha256-cached-foo",
		GoVersion: "1.22",
	}
	barExisting := mod.ModuleConfig{
		Path:      "example.com/bar",
		Version:   "v2.0.0",
		Hash:      "sha256-cached-bar",
		GoVersion: "1.23",
	}

	tests := []struct {
		name                string
		modules             []modulestxt.Module // nil → derived from downloads with no replacements
		downloads           []ModuleDownload
		existingMods        map[string]mod.ModuleConfig
		replacements        map[string]mod.Replacement
		wantHashCalls       int64
		wantHash            string
		wantVersion         string
		wantRequiredVersion string
		wantGoVersion       string
		wantVersioned       bool
		wantUnused          bool
		wantImplicit        bool
		wantReplacedPath    string
		wantPackages        []string
		hasherErr           error
		wantErr             string
		// For cases resolving two modules — asserted unconditionally against
		// deps[1] whenever there are two, so a wrong second entry can't hide
		// behind an unset expectation the way an omitted deps[0] field used to.
		wantVersion2   string
		wantHash2      string
		wantGoVersion2 string
	}{
		{
			name:          "ReusesHashWhenVersionMatches",
			downloads:     []ModuleDownload{fooDownload},
			existingMods:  map[string]mod.ModuleConfig{"example.com/foo": fooExisting},
			wantHashCalls: 0,
			wantHash:      "sha256-cached-foo",
			wantVersion:   "v1.2.3",
			wantGoVersion: "1.22",
		},
		{
			name:          "HashesWhenVersionDiffers",
			downloads:     []ModuleDownload{fooDownload},
			existingMods:  map[string]mod.ModuleConfig{"example.com/foo": {Path: "example.com/foo", Version: "v1.0.0", Hash: "sha256-old"}},
			wantHashCalls: 1,
			wantHash:      "sha256-fresh",
			wantVersion:   "v1.2.3",
		},
		{
			name:          "HashesNewModuleNotInExisting",
			downloads:     []ModuleDownload{barDownload},
			existingMods:  map[string]mod.ModuleConfig{"example.com/foo": fooExisting},
			wantHashCalls: 1,
			wantHash:      "sha256-fresh",
			wantVersion:   "v2.0.0",
		},
		{
			name:           "HashesAllWhenExistingModsIsNil",
			downloads:      []ModuleDownload{fooDownload, barDownload},
			existingMods:   nil,
			wantHashCalls:  2,
			wantHash:       "sha256-fresh",
			wantVersion:    "v1.2.3",
			wantVersion2:   "v2.0.0",
			wantHash2:      "sha256-fresh",
			wantGoVersion2: "",
		},
		{
			name:           "ReusesBothWhenAllMatch",
			downloads:      []ModuleDownload{fooDownload, barDownload},
			existingMods:   map[string]mod.ModuleConfig{"example.com/foo": fooExisting, "example.com/bar": barExisting},
			wantHashCalls:  0,
			wantHash:       "sha256-cached-foo",
			wantVersion:    "v1.2.3",
			wantGoVersion:  "1.22",
			wantVersion2:   "v2.0.0",
			wantHash2:      "sha256-cached-bar",
			wantGoVersion2: "1.23",
		},
		{
			name:      "HashesWhenCachedHashIsEmpty",
			downloads: []ModuleDownload{fooDownload},
			existingMods: map[string]mod.ModuleConfig{"example.com/foo": {
				Path: "example.com/foo", Version: "v1.2.3", Hash: "",
			}},
			wantHashCalls: 1,
			wantHash:      "sha256-fresh",
			wantVersion:   "v1.2.3",
		},
		{
			name:      "NeverReusesLocalEntryEvenIfVersionMatches",
			downloads: []ModuleDownload{fooDownload},
			existingMods: map[string]mod.ModuleConfig{"example.com/foo": {
				Path: "example.com/foo", Version: "v1.2.3", Hash: "sha256-local", Local: "./local/foo",
			}},
			wantHashCalls: 1,
			wantHash:      "sha256-fresh",
			wantVersion:   "v1.2.3",
		},
		{
			// go.mod named a version on the left of "=>" (replace foo vOld => fork
			// vNew): read directly from the replacements map, not inferred from
			// modules.txt — the header is identical whether versioned or not.
			name: "MarksVersionedWhenReplaceDirectiveNamesAVersion",
			modules: []modulestxt.Module{{
				Path:     "example.com/foo",
				Version:  "v1.2.3",
				Explicit: true,
				Replace:  &modulestxt.Replace{Path: "example.com/foo-fork", Version: "v1.2.3"},
			}},
			downloads: []ModuleDownload{{Path: "example.com/foo-fork", Version: "v1.2.3", Dir: "/cache/fork"}},
			replacements: map[string]mod.Replacement{
				"example.com/foo": {OldPath: "example.com/foo", OldVersion: "v1.2.3", NewPath: "example.com/foo-fork"},
			},
			wantHashCalls:    1,
			wantHash:         "sha256-fresh",
			wantVersion:      "v1.2.3",
			wantVersioned:    true,
			wantReplacedPath: "example.com/foo-fork",
		},
		{
			// Same replacement, but go.mod's directive omits the version (replace
			// foo => fork vNew) — the common, unversioned form.
			name: "NotVersionedWhenReplaceDirectiveOmitsVersion",
			modules: []modulestxt.Module{{
				Path:     "example.com/foo",
				Version:  "v1.2.3",
				Explicit: true,
				Replace:  &modulestxt.Replace{Path: "example.com/foo-fork", Version: "v1.2.3"},
			}},
			downloads: []ModuleDownload{{Path: "example.com/foo-fork", Version: "v1.2.3", Dir: "/cache/fork"}},
			replacements: map[string]mod.Replacement{
				"example.com/foo": {OldPath: "example.com/foo", NewPath: "example.com/foo-fork"},
			},
			wantHashCalls:    1,
			wantHash:         "sha256-fresh",
			wantVersion:      "v1.2.3",
			wantVersioned:    false,
			wantReplacedPath: "example.com/foo-fork",
		},
		{
			// An unused wildcard replace: go.mod declares "replace foo => fork
			// vNew", but nothing requires foo, so go mod vendor recorded only the
			// trailer form — modules.txt carries no version for foo. Nothing should
			// be looked up in the download output or hashed; fork's version comes
			// straight from the replace directive.
			name: "UnusedWildcardReplaceSkipsDownloadAndHash",
			modules: []modulestxt.Module{{
				Path:    "example.com/foo",
				Replace: &modulestxt.Replace{Path: "example.com/foo-fork", Version: "v9.9.9"},
			}},
			wantHashCalls:    0,
			wantUnused:       true,
			wantVersion:      "v9.9.9",
			wantReplacedPath: "example.com/foo-fork",
		},
		{
			// An unused *versioned* replace: go.mod declares "replace foo vOld =>
			// fork vNew", but nothing requires foo. Unlike the wildcard case, go
			// mod vendor's trailer-only line still carries foo's old version, so
			// this is only distinguishable from a used, versioned replace by
			// having no packages or annotation — not by an empty Version.
			name: "UnusedVersionedReplaceSkipsDownloadAndHash",
			modules: []modulestxt.Module{{
				Path:    "example.com/foo",
				Version: "v0.8.0",
				Replace: &modulestxt.Replace{Path: "example.com/foo-fork", Version: "v0.9.1"},
			}},
			replacements: map[string]mod.Replacement{
				"example.com/foo": {OldPath: "example.com/foo", OldVersion: "v0.8.0", NewPath: "example.com/foo-fork"},
			},
			wantHashCalls:       0,
			wantUnused:          true,
			wantVersioned:       true,
			wantVersion:         "v0.9.1",
			wantRequiredVersion: "v0.8.0",
			wantReplacedPath:    "example.com/foo-fork",
		},
		{
			// An unused wildcard replace whose replacement target *is* found in
			// the download output — not because foo's replace was exercised, but
			// because something else independently requires fork too. A naive
			// "was the replacement path downloaded" check would misclassify this
			// as used; the entry must still be recognised as unused from its own
			// shape (no packages, no annotation), regardless of fork's status.
			name: "UnusedWildcardReplaceIgnoresUnrelatedDownloadOfReplacementTarget",
			modules: []modulestxt.Module{
				{
					Path:    "example.com/foo",
					Replace: &modulestxt.Replace{Path: "example.com/foo-fork", Version: "v9.9.9"},
				},
				{
					Path:      "example.com/foo-fork",
					Version:   "v9.9.9",
					Explicit:  true,
					GoVersion: "1.22",
					Packages:  []string{"example.com/foo-fork"},
				},
			},
			downloads:        []ModuleDownload{{Path: "example.com/foo-fork", Version: "v9.9.9", Dir: "/cache/fork"}},
			wantHashCalls:    1, // only for the second, genuinely-used module entry
			wantUnused:       true,
			wantVersion:      "v9.9.9",
			wantReplacedPath: "example.com/foo-fork",
			wantVersion2:     "v9.9.9",
			wantHash2:        "sha256-fresh",
			wantGoVersion2:   "1.22",
		},
		{
			// modules.txt shows gopkg.in/ini.v1 => example.com/foo-fork.
			// The existing manifest entry matches — hash is reused.
			name: "ReusesRemoteReplaceEntryWhenReplacedPathMatches",
			modules: []modulestxt.Module{{
				Path:     "example.com/foo",
				Version:  "v1.2.3",
				Explicit: true,
				Replace:  &modulestxt.Replace{Path: "example.com/foo-fork", Version: "v1.2.3"},
			}},
			downloads: []ModuleDownload{{Path: "example.com/foo-fork", Version: "v1.2.3", Dir: "/cache/fork"}},
			existingMods: map[string]mod.ModuleConfig{"example.com/foo": {
				Path: "example.com/foo", Version: "v1.2.3",
				Hash: "sha256-replace-cached", GoVersion: "1.22",
				ReplacedPath: "example.com/foo-fork",
			}},
			wantHashCalls:    0,
			wantHash:         "sha256-replace-cached",
			wantVersion:      "v1.2.3",
			wantGoVersion:    "1.22",
			wantReplacedPath: "example.com/foo-fork",
		},
		{
			// Cached entry has no GoVersion (e.g. schema 3 manifest). Hash is still
			// reused; GoVersion stays empty when the modules.txt annotation has none.
			name:      "GoVersionStaysEmptyWhenCachedEntryAndModulesTxtBothLackIt",
			downloads: []ModuleDownload{fooDownload},
			existingMods: map[string]mod.ModuleConfig{"example.com/foo": {
				Path: "example.com/foo", Version: "v1.2.3", Hash: "sha256-cached-foo",
			}},
			wantHashCalls: 0,
			wantHash:      "sha256-cached-foo",
			wantVersion:   "v1.2.3",
		},
		{
			// Cached entry has no GoVersion but modules.txt carries the annotation.
			// Hash is reused and GoVersion is populated from the modules.txt value.
			name: "PopulatesGoVersionFromModulesTxtWhenCachedEntryHasNone",
			modules: []modulestxt.Module{
				{Path: "example.com/foo", Version: "v1.2.3", Explicit: true, GoVersion: "1.26.0", Packages: []string{"example.com/foo"}},
			},
			downloads: []ModuleDownload{fooDownload},
			existingMods: map[string]mod.ModuleConfig{"example.com/foo": {
				Path: "example.com/foo", Version: "v1.2.3", Hash: "sha256-cached-foo",
			}},
			wantHashCalls: 0,
			wantHash:      "sha256-cached-foo",
			wantVersion:   "v1.2.3",
			wantGoVersion: "1.26.0",
			wantPackages:  []string{"example.com/foo"},
		},
		{
			// The go.mod replace target changed (fork → fork2) since the manifest was
			// cached. The cached ReplacedPath (foo-fork) no longer matches the new
			// target (foo-fork2), so the hash must be recomputed.
			name: "HashesRemoteReplaceWhenReplacedPathChanged",
			modules: []modulestxt.Module{{
				Path:     "example.com/foo",
				Version:  "v1.2.3",
				Explicit: true,
				Replace:  &modulestxt.Replace{Path: "example.com/foo-fork2", Version: "v1.2.3"},
			}},
			downloads: []ModuleDownload{{Path: "example.com/foo-fork2", Version: "v1.2.3", Dir: "/cache/fork2"}},
			existingMods: map[string]mod.ModuleConfig{"example.com/foo": {
				Path: "example.com/foo", Version: "v1.2.3",
				Hash: "sha256-replace-cached", ReplacedPath: "example.com/foo-fork",
			}},
			wantHashCalls:    1,
			wantHash:         "sha256-fresh",
			wantVersion:      "v1.2.3",
			wantReplacedPath: "example.com/foo-fork2",
		},
		{
			// A module vendored without an explicit go.mod requirement (purely
			// transitive) has no "## explicit" annotation in modules.txt.
			// Every other case in this table sets Explicit: true, so without
			// this one, Implicit's mapping from !m.Explicit would never
			// actually be exercised — a regression that always returned
			// Implicit: false would pass every other case here too.
			name: "MarksImplicitWhenModuleLacksExplicitAnnotation",
			modules: []modulestxt.Module{
				{Path: "example.com/foo", Version: "v1.2.3", Explicit: false},
			},
			downloads:     []ModuleDownload{fooDownload},
			wantHashCalls: 1,
			wantHash:      "sha256-fresh",
			wantVersion:   "v1.2.3",
			wantImplicit:  true,
		},
		{
			// modules.txt lists a module (not an unused replace) with no
			// matching entry in the download output — the pool goroutine
			// returns an error, and p.Wait() must propagate it.
			name: "ReturnsErrorWhenDownloadMissing",
			modules: []modulestxt.Module{
				{Path: "example.com/foo", Version: "v1.2.3", Explicit: true},
			},
			wantErr: "module example.com/foo not found in download output",
		},
		{
			// Hasher.Hash fails on the cold path (no reusable cached entry) —
			// the pool goroutine returns a wrapped error, and p.Wait() must
			// propagate it.
			name:      "ReturnsErrorWhenHashFails",
			downloads: []ModuleDownload{fooDownload},
			hasherErr: errors.New("boom"),
			wantErr:   "failed to hash downloaded module example.com/foo@v1.2.3: boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasher := &countingHasher{hash: "sha256-fresh", err: tt.hasherErr}
			r := &Resolver{exec: &fakeExecutor{}, hasher: hasher, reporter: progress.NopReporter{}}

			// Derive plain module entries from downloads when no explicit modules
			// are provided (covers all non-replacement test cases cleanly).
			modules := tt.modules
			if modules == nil {
				modules = make([]modulestxt.Module, 0, len(tt.downloads))
				for _, dl := range tt.downloads {
					modules = append(modules, modulestxt.Module{
						Path:     dl.Path,
						Version:  dl.Version,
						Explicit: true,
					})
				}
			}

			// Hashing now happens via the scheduler, submitted exactly as
			// vendorStreamHandler would submit it from the live stream, then
			// waited on before resolveRemoteModules reads the results back.
			scheduler := newHashScheduler(context.Background(), pool.New(), hasher, progress.NopReporter{}, "test", tt.existingMods, dirResolverFromDownloads(tt.downloads))
			for _, m := range modules {
				if key, ok := remoteContentKey(m); ok {
					scheduler.submit(key)
				}
			}
			scheduler.wait()

			deps, err := r.resolveRemoteModules(modules, scheduler, tt.replacements)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantHashCalls, hasher.count.Load(), "hasher call count")

			// resolveRemoteModules returns exactly one ModuleConfig per input
			// Module when it doesn't error — require this before indexing deps,
			// so a regression that silently drops an entry fails loudly here
			// instead of letting every assertion below skip unnoticed. conker
			// preserves submission order, so deps[i] corresponds to modules[i].
			require.Len(t, deps, len(modules))

			assert.Equal(t, tt.wantHash, deps[0].Hash, "deps[0].Hash")
			assert.Equal(t, tt.wantGoVersion, deps[0].GoVersion, "deps[0].GoVersion")
			assert.Equal(t, tt.wantVersioned, deps[0].Versioned, "deps[0].Versioned")
			assert.Equal(t, tt.wantUnused, deps[0].Unused, "deps[0].Unused")
			assert.Equal(t, tt.wantVersion, deps[0].Version, "deps[0].Version")
			assert.Equal(t, tt.wantRequiredVersion, deps[0].RequiredVersion, "deps[0].RequiredVersion")
			assert.Equal(t, tt.wantImplicit, deps[0].Implicit, "deps[0].Implicit")
			assert.Equal(t, tt.wantReplacedPath, deps[0].ReplacedPath, "deps[0].ReplacedPath")
			assert.Equal(t, tt.wantPackages, deps[0].Packages, "deps[0].Packages")

			// For multi-module cases, verify the second dep unconditionally too —
			// gating on tt.existingMods meant a case with no existing manifest
			// (e.g. HashesAllWhenExistingModsIsNil) never checked its second
			// entry at all, cached or not.
			if len(modules) > 1 {
				assert.Equal(t, tt.wantVersion2, deps[1].Version, "deps[1].Version")
				assert.Equal(t, tt.wantHash2, deps[1].Hash, "deps[1].Hash")
				assert.Equal(t, tt.wantGoVersion2, deps[1].GoVersion, "deps[1].GoVersion")
			}
		})
	}
}

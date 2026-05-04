package resolve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/purpleclay/go-overlay/internal/mod"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeExecutor returns canned responses keyed by either the full command
// string or the first two args (e.g. "go list", "go mod", "git ls-files").
// The responses map is read-only after construction, making fakeExecutor
// safe for concurrent use by the resolver's goroutine pool.
type fakeExecutor struct {
	responses map[string]string
}

func (f *fakeExecutor) Run(args []string, _ string, _ []string) (string, error) {
	full := strings.Join(args, " ")
	if out, ok := f.responses[full]; ok {
		return out, nil
	}

	if len(args) >= 2 {
		key := args[0] + " " + args[1]
		if out, ok := f.responses[key]; ok {
			return out, nil
		}
	}

	return "", fmt.Errorf("unexpected command: %s", full)
}

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestValidatePlatforms(t *testing.T) {
	exec := &fakeExecutor{
		responses: map[string]string{
			"go tool dist list": "linux/amd64\nlinux/arm64\ndarwin/amd64\ndarwin/arm64\n",
		},
	}
	r := New(exec)

	require.NoError(t, r.ValidatePlatforms([]string{"linux/amd64", "darwin/arm64"}))
}

func TestValidatePlatformsRejectsUnsupportedPlatform(t *testing.T) {
	exec := &fakeExecutor{
		responses: map[string]string{
			"go tool dist list": "linux/amd64\nlinux/arm64\ndarwin/amd64\ndarwin/arm64\n",
		},
	}
	r := New(exec)

	err := r.ValidatePlatforms([]string{"plan9/386"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plan9/386")
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

	// testdata/module is used as a stand-in downloaded module directory so
	// NARHash can compute a real hash without needing a real module cache.
	exec := &fakeExecutor{
		responses: map[string]string{
			"go list": `github.com/fatih/color	github.com/fatih/color
github.com/mattn/go-colorable	github.com/mattn/go-colorable
github.com/mattn/go-isatty	github.com/mattn/go-isatty`,
			"go mod": `{"Path":"github.com/fatih/color","Version":"v1.18.0","Dir":"testdata/module","GoMod":"testdata/module/go.mod"}
{"Path":"github.com/mattn/go-colorable","Version":"v0.1.13","Dir":"testdata/module","GoMod":"testdata/module/go.mod"}
{"Path":"github.com/mattn/go-isatty","Version":"v0.0.20","Dir":"testdata/module","GoMod":"testdata/module/go.mod"}`,
		},
	}

	r := New(exec)
	deps, err := r.ResolveModule(goMod, nil)
	require.NoError(t, err)
	require.Len(t, deps, 3)

	// Results must be sorted by module path
	assert.Equal(t, "github.com/fatih/color", deps[0].Path)
	assert.Equal(t, "v1.18.0", deps[0].Version)
	assert.Equal(t, "1.26.0", deps[0].GoVersion) // from testdata/module/go.mod
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
			"go list":      `example.com/localmod	example.com/localmod`,
			"go mod":       "", // no remote downloads
			"git ls-files": "go.mod\nlib.go",
		},
	}

	r := New(exec)
	deps, err := r.ResolveModule(goMod, nil)
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

	// go mod download returns the replacement target path (github.com/go-ini/ini).
	// The resolver must map it back to the original (gopkg.in/ini.v1).
	exec := &fakeExecutor{
		responses: map[string]string{
			"go list": `gopkg.in/ini.v1	gopkg.in/ini.v1`,
			"go mod":  `{"Path":"github.com/go-ini/ini","Version":"v1.67.0","Dir":"testdata/module","GoMod":"testdata/module/go.mod"}`,
		},
	}

	r := New(exec)
	deps, err := r.ResolveModule(goMod, nil)
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

	// Both workspace modules share github.com/fatih/color. The resolver must
	// deduplicate it, merging packages from both modules into a single entry.
	exec := &fakeExecutor{
		responses: map[string]string{
			"go list": `github.com/fatih/color	github.com/fatih/color
github.com/mattn/go-colorable	github.com/mattn/go-colorable
github.com/mattn/go-isatty	github.com/mattn/go-isatty`,
			"go mod": `{"Path":"github.com/fatih/color","Version":"v1.18.0","Dir":"testdata/module","GoMod":"testdata/module/go.mod"}
{"Path":"github.com/mattn/go-colorable","Version":"v0.1.13","Dir":"testdata/module","GoMod":"testdata/module/go.mod"}
{"Path":"github.com/mattn/go-isatty","Version":"v0.0.20","Dir":"testdata/module","GoMod":"testdata/module/go.mod"}`,
		},
	}

	r := New(exec)
	deps, err := r.ResolveWorkspace(goWork, nil)
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

	// cli depends on core, which is also a workspace member. The resolver must
	// clear core's hash and packages since it is resolved from source, not fetched.
	exec := &fakeExecutor{
		responses: map[string]string{
			"go list": `github.com/purpleclay/example/core	github.com/purpleclay/example/core`,
			"go mod":  `{"Path":"github.com/purpleclay/example/core","Version":"v1.0.0","Dir":"testdata/module","GoMod":"testdata/module/go.mod"}`,
		},
	}

	r := New(exec)
	deps, err := r.ResolveWorkspace(goWork, nil)
	require.NoError(t, err)
	require.Len(t, deps, 1)

	assert.Equal(t, "github.com/purpleclay/example/core", deps[0].Path)
	assert.Empty(t, deps[0].Hash)
	assert.Empty(t, deps[0].Packages)
}

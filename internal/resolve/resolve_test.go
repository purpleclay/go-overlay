package resolve

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/purpleclay/go-overlay/internal/mod"
	"github.com/purpleclay/go-overlay/internal/modulestxt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingHasher counts Hash calls and returns a fixed or per-dir hash.
// Safe for concurrent use by the goroutine pool in resolveRemoteModules.
type countingHasher struct {
	count atomic.Int64
	// returned for every call; defaults to "sha256-"+dir
	hash string
}

func (h *countingHasher) Hash(dir string) (string, error) {
	h.count.Add(1)
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
}

func (f *fakeExecutor) Run(_ context.Context, args []string, _ string, _ []string) (string, error) {
	full := strings.Join(args, " ")
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

	require.NoError(t, r.ValidatePlatforms(context.Background(), []string{"linux/amd64", "darwin/arm64"}))
}

func TestValidatePlatformsRejectsUnsupportedPlatform(t *testing.T) {
	exec := &fakeExecutor{
		responses: map[string]string{
			"go tool dist list": "linux/amd64\nlinux/arm64\ndarwin/amd64\ndarwin/arm64\n",
		},
	}
	r := New(exec)

	err := r.ValidatePlatforms(context.Background(), []string{"plan9/386"})
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
			"go mod vendor": `# github.com/fatih/color v1.18.0
## explicit; go 1.25.0
github.com/fatih/color
# github.com/mattn/go-colorable v0.1.13
## explicit; go 1.25.0
github.com/mattn/go-colorable
# github.com/mattn/go-isatty v0.0.20
## explicit; go 1.25.0
github.com/mattn/go-isatty`,
			"go mod": `{"Path":"github.com/fatih/color","Version":"v1.18.0","Dir":"testdata/module","GoMod":"testdata/module/go.mod"}
{"Path":"github.com/mattn/go-colorable","Version":"v0.1.13","Dir":"testdata/module","GoMod":"testdata/module/go.mod"}
{"Path":"github.com/mattn/go-isatty","Version":"v0.0.20","Dir":"testdata/module","GoMod":"testdata/module/go.mod"}`,
		},
	}

	r := New(exec)
	deps, err := r.ResolveModule(context.Background(), goMod, nil, nil)
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
	deps, err := r.ResolveModule(context.Background(), goMod, nil, nil)
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
	// The resolver must map it back to the original (gopkg.in/ini.v1) via
	// the Replace field in modules.txt.
	exec := &fakeExecutor{
		responses: map[string]string{
			"go mod vendor": `# gopkg.in/ini.v1 v1.67.0 => github.com/go-ini/ini v1.67.0
## explicit; go 1.14
gopkg.in/ini.v1`,
			"go mod": `{"Path":"github.com/go-ini/ini","Version":"v1.67.0","Dir":"testdata/module","GoMod":"testdata/module/go.mod"}`,
		},
	}

	r := New(exec)
	deps, err := r.ResolveModule(context.Background(), goMod, nil, nil)
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
			"go mod": `{"Path":"github.com/fatih/color","Version":"v1.18.0","Dir":"testdata/module","GoMod":"testdata/module/go.mod"}
{"Path":"github.com/mattn/go-colorable","Version":"v0.1.13","Dir":"testdata/module","GoMod":"testdata/module/go.mod"}
{"Path":"github.com/mattn/go-isatty","Version":"v0.0.20","Dir":"testdata/module","GoMod":"testdata/module/go.mod"}`,
		},
	}

	r := New(exec)
	deps, err := r.ResolveWorkspace(context.Background(), goWork, nil, nil)
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
	// appear in modules.txt (workspace members are never vendored), but it
	// does appear in go mod download output. The resolver must emit it as a
	// local source entry with empty hash and packages.
	exec := &fakeExecutor{
		responses: map[string]string{
			"go work vendor": `## workspace`,
			"go mod":         `{"Path":"github.com/purpleclay/example/core","Version":"v1.0.0","Dir":"testdata/module","GoMod":"testdata/module/go.mod"}`,
		},
	}

	r := New(exec)
	deps, err := r.ResolveWorkspace(context.Background(), goWork, nil, nil)
	require.NoError(t, err)
	require.Len(t, deps, 1)

	assert.Equal(t, "github.com/purpleclay/example/core", deps[0].Path)
	assert.Empty(t, deps[0].Hash)
	assert.Empty(t, deps[0].Packages)
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
		name               string
		modules            []modulestxt.Module // nil → derived from downloads with no replacements
		downloads          []ModuleDownload
		existingMods       map[string]mod.ModuleConfig
		wantHashCalls      int64
		wantHash           string
		wantGoVersion      string
		wantGoVersionEmpty bool
	}{
		{
			name:          "ReusesHashWhenVersionMatches",
			downloads:     []ModuleDownload{fooDownload},
			existingMods:  map[string]mod.ModuleConfig{"example.com/foo": fooExisting},
			wantHashCalls: 0,
			wantHash:      "sha256-cached-foo",
			wantGoVersion: "1.22",
		},
		{
			name:          "HashesWhenVersionDiffers",
			downloads:     []ModuleDownload{fooDownload},
			existingMods:  map[string]mod.ModuleConfig{"example.com/foo": {Path: "example.com/foo", Version: "v1.0.0", Hash: "sha256-old"}},
			wantHashCalls: 1,
		},
		{
			name:          "HashesNewModuleNotInExisting",
			downloads:     []ModuleDownload{barDownload},
			existingMods:  map[string]mod.ModuleConfig{"example.com/foo": fooExisting},
			wantHashCalls: 1,
		},
		{
			name:          "HashesAllWhenExistingModsIsNil",
			downloads:     []ModuleDownload{fooDownload, barDownload},
			existingMods:  nil,
			wantHashCalls: 2,
		},
		{
			name:          "ReusesBothWhenAllMatch",
			downloads:     []ModuleDownload{fooDownload, barDownload},
			existingMods:  map[string]mod.ModuleConfig{"example.com/foo": fooExisting, "example.com/bar": barExisting},
			wantHashCalls: 0,
			wantHash:      "sha256-cached-foo",
			wantGoVersion: "1.22",
		},
		{
			name:      "HashesWhenCachedHashIsEmpty",
			downloads: []ModuleDownload{fooDownload},
			existingMods: map[string]mod.ModuleConfig{"example.com/foo": {
				Path: "example.com/foo", Version: "v1.2.3", Hash: "",
			}},
			wantHashCalls: 1,
		},
		{
			name:      "NeverReusesLocalEntryEvenIfVersionMatches",
			downloads: []ModuleDownload{fooDownload},
			existingMods: map[string]mod.ModuleConfig{"example.com/foo": {
				Path: "example.com/foo", Version: "v1.2.3", Hash: "sha256-local", Local: "./local/foo",
			}},
			wantHashCalls: 1,
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
			wantHashCalls: 0,
			wantHash:      "sha256-replace-cached",
			wantGoVersion: "1.22",
		},
		{
			// Cached entry has no GoVersion (e.g. schema 3 manifest). Hash is still
			// reused; GoVersion stays empty when the modules.txt annotation has none.
			name:      "GoVersionStaysEmptyWhenCachedEntryAndModulesTxtBothLackIt",
			downloads: []ModuleDownload{fooDownload},
			existingMods: map[string]mod.ModuleConfig{"example.com/foo": {
				Path: "example.com/foo", Version: "v1.2.3", Hash: "sha256-cached-foo",
			}},
			wantHashCalls:      0,
			wantHash:           "sha256-cached-foo",
			wantGoVersionEmpty: true,
		},
		{
			// Cached entry has no GoVersion but modules.txt carries the annotation.
			// Hash is reused and GoVersion is populated from the modules.txt value.
			name: "PopulatesGoVersionFromModulesTxtWhenCachedEntryHasNone",
			modules: []modulestxt.Module{
				{Path: "example.com/foo", Version: "v1.2.3", Explicit: true, GoVersion: "1.26.0"},
			},
			downloads: []ModuleDownload{fooDownload},
			existingMods: map[string]mod.ModuleConfig{"example.com/foo": {
				Path: "example.com/foo", Version: "v1.2.3", Hash: "sha256-cached-foo",
			}},
			wantHashCalls: 0,
			wantHash:      "sha256-cached-foo",
			wantGoVersion: "1.26.0",
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
			wantHashCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasher := &countingHasher{hash: "sha256-fresh"}
			r := &Resolver{exec: &fakeExecutor{}, hasher: hasher}

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

			deps, err := r.resolveRemoteModules(context.Background(), modules, tt.downloads, tt.existingMods)
			require.NoError(t, err)
			assert.Equal(t, tt.wantHashCalls, hasher.count.Load(), "hasher call count")

			if len(deps) > 0 {
				if tt.wantHash != "" {
					assert.Equal(t, tt.wantHash, deps[0].Hash, "deps[0].Hash")
				}
				if tt.wantGoVersion != "" {
					assert.Equal(t, tt.wantGoVersion, deps[0].GoVersion, "deps[0].GoVersion")
				}
				if tt.wantGoVersionEmpty {
					assert.Empty(t, deps[0].GoVersion, "deps[0].GoVersion")
				}
			}
			// For multi-module cases, verify the second dep's cached values too.
			if len(deps) > 1 && tt.existingMods != nil {
				entry := tt.existingMods[deps[1].Path]
				if entry.Hash != "" {
					assert.Equal(t, entry.Hash, deps[1].Hash, "deps[1].Hash")
				}
				if entry.GoVersion != "" {
					assert.Equal(t, entry.GoVersion, deps[1].GoVersion, "deps[1].GoVersion")
				}
			}
		})
	}
}

package mod_test

import (
	"path/filepath"
	"testing"

	"github.com/purpleclay/go-overlay/internal/mod"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const goWorkFile = "go.work"

func TestParseGoWorkFile(t *testing.T) {
	goWork := `
go 1.25.4

toolchain go1.25.4

use (
	./cli
	./core
)
`
	cli := `
module github.com/purpleclay/example/cli

go 1.25.4
`
	core := `
module github.com/purpleclay/example/core

go 1.25.4
`
	dir := t.TempDir()
	writeFile(t, dir, goWorkFile, goWork)
	writeFile(t, dir, "cli/go.mod", cli)
	writeFile(t, dir, "core/go.mod", core)

	gw, err := mod.ParseGoWorkFile(filepath.Join(dir, goWorkFile))
	require.NoError(t, err)

	assert.Equal(t, dir, gw.Dir)
	assert.Equal(t, "1.25.4", gw.GoVersion)
	assert.Equal(t, "go1.25.4", gw.Toolchain)
	assert.ElementsMatch(t, []string{"./cli", "./core"}, gw.Modules)

	cfg := gw.WorkspaceConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, "1.25.4", cfg.Go)
	assert.Equal(t, "go1.25.4", cfg.Toolchain)
	assert.Equal(t, []string{"./cli", "./core"}, cfg.Modules)
}

func TestParseGoWorkFileMembers(t *testing.T) {
	goWork := `
go 1.25.4

use (
	./cli
	./core
)
`
	cli := `
module github.com/purpleclay/example/cli

go 1.25.4
`
	core := `
module github.com/purpleclay/example/core

go 1.25.4
`
	dir := t.TempDir()
	writeFile(t, dir, goWorkFile, goWork)
	writeFile(t, dir, "cli/go.mod", cli)
	writeFile(t, dir, "core/go.mod", core)

	gw, err := mod.ParseGoWorkFile(filepath.Join(dir, goWorkFile))
	require.NoError(t, err)

	members, err := gw.ParseMembers()
	require.NoError(t, err)
	require.Len(t, members, 2)

	byPath := make(map[string]mod.WorkspaceMember, len(members))
	for _, m := range members {
		byPath[m.ModulePath] = m
	}

	assert.Equal(t, "./cli", byPath["github.com/purpleclay/example/cli"].Dir)
	assert.Equal(t, "./core", byPath["github.com/purpleclay/example/core"].Dir)
}

func TestParseGoWorkFileReplacements(t *testing.T) {
	goWork := `
go 1.25.4

use ./cli

replace github.com/old/pkg => github.com/new/pkg v1.2.0
replace example.com/local => ./local
`
	cli := `
module github.com/purpleclay/example/cli

go 1.25.4
`
	dir := t.TempDir()
	writeFile(t, dir, goWorkFile, goWork)
	writeFile(t, dir, "cli/go.mod", cli)

	gw, err := mod.ParseGoWorkFile(filepath.Join(dir, goWorkFile))
	require.NoError(t, err)

	require.Len(t, gw.Replacements, 2)

	remote := gw.RemoteReplacements()
	require.Len(t, remote, 1)
	assert.Equal(t, "github.com/old/pkg", remote["github.com/new/pkg"].OldPath)
	assert.Equal(t, "github.com/new/pkg", remote["github.com/new/pkg"].NewPath)
	assert.False(t, remote["github.com/new/pkg"].IsLocal)

	local := gw.LocalReplacements()
	require.Len(t, local, 1)
	assert.Equal(t, "example.com/local", local[0].OldPath)
	assert.Equal(t, "./local", local[0].LocalPath)
	assert.True(t, local[0].IsLocal)
}

func TestParseMembersReturnsErrorForMissingModuleDirective(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, goWorkFile, `
go 1.25.4

use (
	./cli
)
`)
	writeFile(t, dir, "cli/go.mod", `go 1.25.4
`)

	gw, err := mod.ParseGoWorkFile(filepath.Join(dir, goWorkFile))
	require.NoError(t, err)

	_, err = gw.ParseMembers()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing a module directive")
}

func TestNewGoWorkFileFromManifest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "cli/go.mod", "module github.com/purpleclay/example/cli\n\ngo 1.25.4\n")
	writeFile(t, dir, "core/go.mod", "module github.com/purpleclay/example/core\n\ngo 1.25.4\n")

	cfg := &mod.WorkspaceConfig{
		Go:        "1.25.4",
		Toolchain: "go1.25.4",
		Modules:   []string{"./cli", "./core"},
	}

	gw, err := mod.NewGoWorkFileFromManifest(dir, cfg)
	require.NoError(t, err)
	assert.Equal(t, dir, gw.Dir)
	assert.Equal(t, cfg, gw.WorkspaceConfig())
}

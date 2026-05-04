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

	assert.Equal(t, dir, gw.Dir())
	assert.Equal(t, "sha256-IJpcpXwkPQuqU/YHXlMIy9fNLkSzCfTfw6HRtzSBdok=", gw.Hash())
	assert.ElementsMatch(t, []string{"./cli", "./core"}, gw.Modules())

	cfg := gw.WorkspaceConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, "1.25.4", cfg.Go)
	assert.Equal(t, "go1.25.4", cfg.Toolchain)
	assert.Equal(t, []string{"./cli", "./core"}, cfg.Modules)

	members, err := gw.WorkspaceModulePaths()
	require.NoError(t, err)
	assert.Equal(t, "./cli", members["github.com/purpleclay/example/cli"])
	assert.Equal(t, "./core", members["github.com/purpleclay/example/core"])
}

func TestGoWorkFileHashStableUnderModuleOrder(t *testing.T) {
	cli := `
module github.com/purpleclay/example/cli

go 1.25.4
`
	core := `
module github.com/purpleclay/example/core

go 1.25.4
`

	dir1 := t.TempDir()
	writeFile(t, dir1, goWorkFile, `
go 1.25.4

use (
	./cli
	./core
)
`)
	writeFile(t, dir1, "cli/go.mod", cli)
	writeFile(t, dir1, "core/go.mod", core)

	dir2 := t.TempDir()
	writeFile(t, dir2, goWorkFile, `
go 1.25.4

use (
	./core
	./cli
)
`)
	writeFile(t, dir2, "cli/go.mod", cli)
	writeFile(t, dir2, "core/go.mod", core)

	gw1, err := mod.ParseGoWorkFile(filepath.Join(dir1, goWorkFile))
	require.NoError(t, err)
	gw2, err := mod.ParseGoWorkFile(filepath.Join(dir2, goWorkFile))
	require.NoError(t, err)

	assert.Equal(t, gw1.Hash(), gw2.Hash())
}

func TestWorkspaceModulePathsReturnsErrorForMissingModuleDirective(t *testing.T) {
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

	_, err = gw.WorkspaceModulePaths()
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
	assert.Equal(t, dir, gw.Dir())
	assert.NotEmpty(t, gw.Hash())
	assert.Equal(t, cfg, gw.WorkspaceConfig())
}

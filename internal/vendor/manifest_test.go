package vendor_test

import (
	"bytes"
	"testing"

	"github.com/purpleclay/go-overlay/internal/mod"
	"github.com/purpleclay/go-overlay/internal/vendor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	data := []byte(`schema = 2
hash = "sha256-7rwfLDKKRjrYZVf/fGtIiL45LOWF5S05WW5yvDESJxE="

[mod]
  [mod."github.com/BurntSushi/toml"]
    version = "v1.6.0"
    hash = "sha256-dRaEfpa2VI55EwlIW72hMRHdWouJeRF7TPYhI+AUQjk="
    go = "1.18"
    packages = ["github.com/BurntSushi/toml"]`)

	m, err := vendor.Parse(data)
	require.NoError(t, err)

	assert.Equal(t, 2, m.Schema)
	assert.Equal(t, "sha256-7rwfLDKKRjrYZVf/fGtIiL45LOWF5S05WW5yvDESJxE=", m.Hash)
	assert.Nil(t, m.Workspace)
	require.Len(t, m.Mod, 1)

	dep := m.Mod["github.com/BurntSushi/toml"]
	assert.Equal(t, "github.com/BurntSushi/toml", dep.Path)
	assert.Equal(t, "v1.6.0", dep.Version)
	assert.Equal(t, "sha256-dRaEfpa2VI55EwlIW72hMRHdWouJeRF7TPYhI+AUQjk=", dep.Hash)
	assert.Equal(t, "1.18", dep.GoVersion)
	assert.Equal(t, []string{"github.com/BurntSushi/toml"}, dep.Packages)
}

func TestParseWithWorkspace(t *testing.T) {
	data := []byte(`schema = 2
hash = "sha256-IJpcpXwkPQuqU/YHXlMIy9fNLkSzCfTfw6HRtzSBdok="

[workspace]
  go = "1.25.4"
  toolchain = "go1.25.4"
  modules = ["./cli", "./core"]

[mod]
  [mod."github.com/fatih/color"]
    version = "v1.18.0"
    hash = "sha256-pP5y72FSbi4j/BjyVq/XbAOFjzNjMxZt2R/lFFxGWvY="
    go = "1.17"
    packages = ["github.com/fatih/color"]`)

	m, err := vendor.Parse(data)
	require.NoError(t, err)

	assert.Equal(t, 2, m.Schema)
	assert.Equal(t, "sha256-IJpcpXwkPQuqU/YHXlMIy9fNLkSzCfTfw6HRtzSBdok=", m.Hash)
	require.NotNil(t, m.Workspace)
	assert.Equal(t, "1.25.4", m.Workspace.Go)
	assert.Equal(t, "go1.25.4", m.Workspace.Toolchain)
	assert.Equal(t, []string{"./cli", "./core"}, m.Workspace.Modules)
	require.Len(t, m.Mod, 1)

	dep := m.Mod["github.com/fatih/color"]
	assert.Equal(t, "v1.18.0", dep.Version)
	assert.Equal(t, "sha256-pP5y72FSbi4j/BjyVq/XbAOFjzNjMxZt2R/lFFxGWvY=", dep.Hash)
	assert.Equal(t, "1.17", dep.GoVersion)
	assert.Equal(t, []string{"github.com/fatih/color"}, dep.Packages)
}

func TestWriteTo(t *testing.T) {
	manifest := vendor.New(
		"sha256-7rwfLDKKRjrYZVf/fGtIiL45LOWF5S05WW5yvDESJxE=",
		[]mod.ModuleConfig{
			{
				Path:      "github.com/BurntSushi/toml",
				Version:   "v1.6.0",
				Hash:      "sha256-dRaEfpa2VI55EwlIW72hMRHdWouJeRF7TPYhI+AUQjk=",
				GoVersion: "1.18",
				Packages:  []string{"github.com/BurntSushi/toml"},
			},
		},
		[]string{"linux/amd64"},
		&mod.WorkspaceConfig{
			Go:        "1.25.4",
			Toolchain: "go1.25.4",
			Modules:   []string{"./cli"},
		},
	)

	var buf bytes.Buffer
	n, err := manifest.WriteTo(&buf)
	require.NoError(t, err)
	assert.Equal(t, int64(buf.Len()), n)

	parsed, err := vendor.Parse(buf.Bytes())
	require.NoError(t, err)
	assert.Equal(t, manifest.Schema, parsed.Schema)
	assert.Equal(t, manifest.Hash, parsed.Hash)
	assert.Equal(t, manifest.IncludePlatforms, parsed.IncludePlatforms)
	assert.Equal(t, manifest.Workspace, parsed.Workspace)
	assert.Equal(t, manifest.Mod, parsed.Mod)
}

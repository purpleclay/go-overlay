package manifest_test

import (
	"bytes"
	"testing"

	"github.com/purpleclay/go-overlay/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	data := []byte(`schema = 3

[mod]
  [mod."github.com/BurntSushi/toml"]
    version = "v1.6.0"
    hash = "sha256-dRaEfpa2VI55EwlIW72hMRHdWouJeRF7TPYhI+AUQjk="
    go = "1.18"
    packages = ["github.com/BurntSushi/toml"]`)

	m, err := manifest.Parse(data)
	require.NoError(t, err)

	assert.Equal(t, 3, m.Schema)
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
	data := []byte(`schema = 3

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

	m, err := manifest.Parse(data)
	require.NoError(t, err)

	assert.Equal(t, 3, m.Schema)
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

func TestParseValidation(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr string
	}{
		{
			name:    "MissingSchema",
			data:    `[mod]`,
			wantErr: "missing required 'schema' field",
		},
		{
			name:    "MissingModSection",
			data:    `schema = 3`,
			wantErr: "missing required '[mod]' section",
		},
		{
			name: "WorkspaceMissingGo",
			data: `schema = 3

[workspace]
  modules = ["./api"]

[mod]`,
			wantErr: "[workspace] missing required 'go' field",
		},
		{
			name: "WorkspaceMissingModules",
			data: `schema = 3

[workspace]
  go = "1.26.0"

[mod]`,
			wantErr: "[workspace] missing required 'modules' field",
		},
		{
			name: "ToolMissingVersion",
			data: `schema = 3

[tool]
  [tool."golang.org/x/tools/cmd/stringer"]

[mod]`,
			wantErr: "missing required 'version' field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manifest.Parse([]byte(tt.data))
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestWriteTo(t *testing.T) {
	m := manifest.New(
		[]manifest.Module{
			{
				Path:      "github.com/BurntSushi/toml",
				Version:   "v1.6.0",
				Hash:      "sha256-dRaEfpa2VI55EwlIW72hMRHdWouJeRF7TPYhI+AUQjk=",
				GoVersion: "1.18",
				Packages:  []string{"github.com/BurntSushi/toml"},
			},
		},
		&manifest.Workspace{
			Go:        "1.25.4",
			Toolchain: "go1.25.4",
			Modules:   []string{"./cli"},
		},
		manifest.Tools{"github.com/a-h/templ/cmd/templ": {Version: "v0.3.924"}},
		nil,
	)

	var buf bytes.Buffer
	n, err := m.WriteTo(&buf)
	require.NoError(t, err)
	assert.Equal(t, int64(buf.Len()), n)

	parsed, err := manifest.Parse(buf.Bytes())
	require.NoError(t, err)
	assert.Equal(t, m.Schema, parsed.Schema)
	assert.Equal(t, m.Workspace, parsed.Workspace)
	assert.Equal(t, m.Tool, parsed.Tool)
	assert.Equal(t, m.Mod, parsed.Mod)
}

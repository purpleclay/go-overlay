package vendor_test

import (
	"bytes"
	"testing"

	"github.com/purpleclay/go-overlay/internal/vendor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/golden"
)

func TestParse(t *testing.T) {
	t.Run("BackfillsModulePath", func(t *testing.T) {
		data := []byte(`schema = 2
hash = "sha256-abc123"

[mod]
  [mod."github.com/foo/bar"]
    version = "v1.0.0"
    hash = "sha256-def456"`)
		m, err := vendor.Parse(data)
		require.NoError(t, err)
		cfg, ok := m.Mod["github.com/foo/bar"]
		require.True(t, ok)
		assert.Equal(t, "github.com/foo/bar", cfg.Path)
	})
}

func TestWriteTo(t *testing.T) {
	tests := []struct {
		name             string
		hash             string
		deps             []vendor.ModuleConfig
		includePlatforms []string
		workspace        *vendor.WorkspaceConfig
	}{
		{
			name: "simple",
			hash: "sha256-modulehash=",
			deps: []vendor.ModuleConfig{
				{
					Path:      "github.com/stretchr/testify",
					Version:   "v1.9.0",
					Hash:      "sha256-abc123def456=",
					GoVersion: "1.20",
					Packages:  []string{"github.com/stretchr/testify/assert"},
				},
				{
					Path:     "github.com/davecgh/go-spew",
					Version:  "v1.1.1",
					Hash:     "sha256-xyz789=",
					Packages: []string{"github.com/davecgh/go-spew/spew"},
				},
			},
		},
		{
			name: "with-platforms",
			hash: "sha256-modulehash=",
			deps: []vendor.ModuleConfig{
				{
					Path:      "github.com/stretchr/testify",
					Version:   "v1.9.0",
					Hash:      "sha256-abc123def456=",
					GoVersion: "1.20",
					Packages:  []string{"github.com/stretchr/testify/assert"},
				},
			},
			includePlatforms: []string{"freebsd/amd64", "freebsd/arm64"},
		},
		{
			name: "workspace",
			hash: "sha256-workspacehash=",
			deps: []vendor.ModuleConfig{
				{
					Path:         "example.com/shared",
					Version:      "v0.0.0",
					GoVersion:    "1.22",
					ReplacedPath: "example.com/shared",
					Local:        "./shared",
				},
			},
			workspace: &vendor.WorkspaceConfig{
				Go:            "1.22",
				ModuleConfigs: []string{"./api", "./shared"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := vendor.New(tt.hash, tt.deps, tt.includePlatforms, tt.workspace)

			var buf bytes.Buffer
			n, err := manifest.WriteTo(&buf)
			require.NoError(t, err)
			require.Equal(t, int64(buf.Len()), n)

			golden.Assert(t, buf.String(), tt.name+"/govendor.golden")
		})
	}
}

package mod

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/golden"
)

func TestExtractSchema(t *testing.T) {
	t.Run("CurrentSchema", func(t *testing.T) {
		data := []byte(`schema = 2`)
		schema, err := extractSchema(data)
		require.NoError(t, err)
		assert.Equal(t, schemaVersion, schema)
	})

	t.Run("OldSchema", func(t *testing.T) {
		data := []byte(`schema = 1`)
		schema, err := extractSchema(data)
		require.NoError(t, err)
		assert.Equal(t, 1, schema)
	})

	t.Run("MissingSchema", func(t *testing.T) {
		data := []byte(`hash = "sha256-abc123"`)
		schema, err := extractSchema(data)
		require.NoError(t, err)
		assert.Equal(t, 0, schema)
	})
}

func TestManifestWriteTo(t *testing.T) {
	tests := []struct {
		name             string
		dir              string
		includePlatforms []string
	}{
		{
			name: "simple",
			dir:  "testdata/simple",
		},
		{
			name:             "with-platforms",
			dir:              "testdata/with-platforms",
			includePlatforms: []string{"freebsd/amd64", "freebsd/arm64"},
		},
		{
			name: "local-replace",
			dir:  "testdata/local-replace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goModPath := filepath.Join(tt.dir, "go.mod")
			goMod, err := ParseGoModFile(goModPath)
			require.NoError(t, err)

			platforms := DefaultPlatforms
			if len(tt.includePlatforms) > 0 {
				platforms = append(DefaultPlatforms, tt.includePlatforms...)
			}
			manifest, err := newManifest(goMod, platforms, tt.includePlatforms)
			require.NoError(t, err)

			var buf bytes.Buffer
			if _, err := manifest.WriteTo(&buf); err != nil {
				require.NoError(t, err)
			}

			golden.Assert(t, buf.String(), tt.name+"/govendor.golden")
		})
	}
}

func TestWorkspaceManifestWriteTo(t *testing.T) {
	tests := []struct {
		name             string
		dir              string
		includePlatforms []string
	}{
		{
			name: "workspace",
			dir:  "testdata/workspace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goWorkPath := filepath.Join(tt.dir, "go.work")
			goWork, err := ParseGoWorkFile(goWorkPath)
			require.NoError(t, err)

			platforms := DefaultPlatforms
			if len(tt.includePlatforms) > 0 {
				platforms = append(DefaultPlatforms, tt.includePlatforms...)
			}
			manifest, err := newWorkspaceManifest(goWork, platforms, tt.includePlatforms)
			require.NoError(t, err)

			var buf bytes.Buffer
			if _, err := manifest.WriteTo(&buf); err != nil {
				require.NoError(t, err)
			}

			golden.Assert(t, buf.String(), tt.name+"/govendor.golden")
		})
	}
}

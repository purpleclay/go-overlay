package mod

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/purpleclay/go-overlay/internal/vendor"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/golden"
)

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
		{
			name: "remote-replace",
			dir:  "testdata/remote-replace",
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
			deps, err := goMod.Dependencies(platforms)
			require.NoError(t, err)

			manifest := vendor.New(goMod.Hash(), deps, tt.includePlatforms, nil)

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
			deps, err := goWork.Dependencies(platforms)
			require.NoError(t, err)

			manifest := vendor.New(goWork.Hash(), deps, tt.includePlatforms, goWork.WorkspaceConfig())

			var buf bytes.Buffer
			if _, err := manifest.WriteTo(&buf); err != nil {
				require.NoError(t, err)
			}

			golden.Assert(t, buf.String(), tt.name+"/govendor.golden")
		})
	}
}

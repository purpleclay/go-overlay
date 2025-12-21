package mod

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/v3/golden"
)

func TestManifestWriteTo(t *testing.T) {
	tests := []struct {
		name string
		dir  string
	}{
		{
			name: "simple",
			dir:  "testdata/simple",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goModPath := filepath.Join(tt.dir, "go.mod")
			goMod, err := ParseGoModFile(goModPath)
			require.NoError(t, err)

			manifest, err := newManifest(goMod)
			require.NoError(t, err)

			var buf bytes.Buffer
			if _, err := manifest.WriteTo(&buf); err != nil {
				require.NoError(t, err)
			}

			golden.Assert(t, buf.String(), tt.name+"/govendor.golden")
		})
	}
}

package resolve_test

import (
	"path/filepath"
	"testing"

	"github.com/nix-community/go-nix/pkg/nar"
	"github.com/purpleclay/go-overlay/internal/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNARHash(t *testing.T) {
	hash, err := resolve.NARHash("testdata/module")
	require.NoError(t, err)
	assert.Equal(t, "sha256-YX+5OWGKiEJOoSfTZ5aXG1RhWHyDgKeCygqVZny98JU=", hash)
}

func TestNARHashFiltered(t *testing.T) {
	hash, err := resolve.NARHashFiltered("testdata/module", func(path string, _ nar.NodeType) bool {
		return filepath.Base(path) != "main.go"
	})
	require.NoError(t, err)
	assert.Equal(t, "sha256-OtzhFGcov9e8CDxmuKGQIlxXHLMKclkhjgekIe56Uks=", hash)
}

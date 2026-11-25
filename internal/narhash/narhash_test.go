package narhash_test

import (
	"path/filepath"
	"testing"

	"github.com/nix-community/go-nix/pkg/nar"
	"github.com/purpleclay/go-overlay/internal/narhash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDir(t *testing.T) {
	hash, err := narhash.Dir("testdata/module")
	require.NoError(t, err)
	assert.Equal(t, "sha256-zlKFPywupKwyb/YxUrm8G75PLGt147OLoyzCt0Sv+XM=", hash)
}

func TestDirFiltered(t *testing.T) {
	hash, err := narhash.DirFiltered("testdata/module", func(path string, _ nar.NodeType) bool {
		return filepath.Base(path) != "main.go"
	})
	require.NoError(t, err)
	assert.Equal(t, "sha256-2vpSQCJnikMhONDkJruOc4w3uJmwdHgUGRk4l8FZXgQ=", hash)
}

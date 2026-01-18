package mod

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindWorkspaceManifestReturnsEmptyIfNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "cmd", "api")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	result, err := FindWorkspaceManifest(filepath.Join("cmd", "api", "go.mod"))

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestFindWorkspaceManifestTraversesUp(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "cmd", "api")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	manifestFile := filepath.Join(tmpDir, "govendor.toml")
	require.NoError(t, os.WriteFile(manifestFile, []byte("test"), 0o644))

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	result, err := FindWorkspaceManifest(filepath.Join("cmd", "api", "go.mod"))

	require.NoError(t, err)
	expected, _ := filepath.EvalSymlinks(manifestFile)
	assert.Equal(t, expected, result)
}

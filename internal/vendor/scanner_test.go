package vendor_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/purpleclay/go-overlay/internal/vendor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeModFile(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("module example.com/test\n\ngo 1.25.4\n"), 0o644))
}

func TestScanFrom(t *testing.T) {
	dir := t.TempDir()
	writeModFile(t, dir, "api/go.mod")
	writeModFile(t, dir, "cmd/cli/go.mod")
	writeModFile(t, dir, "internal/core/go.mod")

	scanner := vendor.NewFileTreeScanner()
	paths, err := scanner.ScanFrom(dir)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		filepath.Join(dir, "api", "go.mod"),
		filepath.Join(dir, "cmd", "cli", "go.mod"),
		filepath.Join(dir, "internal", "core", "go.mod"),
	}, paths)
}

func TestScanFromReturnsEmptyWhenNoModFilesFound(t *testing.T) {
	dir := t.TempDir()

	scanner := vendor.NewFileTreeScanner()
	paths, err := scanner.ScanFrom(dir)
	require.NoError(t, err)
	assert.Empty(t, paths)
}

func TestScanFromSkipsKnownDirectories(t *testing.T) {
	dir := t.TempDir()
	writeModFile(t, dir, "api/go.mod")
	writeModFile(t, dir, ".git/go.mod")
	writeModFile(t, dir, "vendor/go.mod")
	writeModFile(t, dir, "node_modules/go.mod")
	writeModFile(t, dir, "testdata/go.mod")

	scanner := vendor.NewFileTreeScanner()
	paths, err := scanner.ScanFrom(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join(dir, "api", "go.mod")}, paths)
}

func TestScanFromWithMaxDepth(t *testing.T) {
	dir := t.TempDir()
	writeModFile(t, dir, "api/go.mod")     // 1 directory deep — found with MaxDepth=2
	writeModFile(t, dir, "cmd/cli/go.mod") // 2 directories deep — not found with MaxDepth=2

	scanner := vendor.NewFileTreeScanner(vendor.WithMaxDepth(2))
	paths, err := scanner.ScanFrom(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join(dir, "api", "go.mod")}, paths)
}

func TestFindWorkspaceManifestReturnsEmptyIfNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "cmd", "api")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer func() {
		require.NoError(t, os.Chdir(origDir))
	}()

	result, err := vendor.FindWorkspaceManifest(filepath.Join("cmd", "api", "go.mod"))
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestFindWorkspaceManifestTraversesUp(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "cmd", "api")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	manifestFile := filepath.Join(tmpDir, "govendor.toml")
	require.NoError(t, os.WriteFile(manifestFile, []byte("test"), 0o644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer func() {
		require.NoError(t, os.Chdir(origDir))
	}()

	result, err := vendor.FindWorkspaceManifest(filepath.Join("cmd", "api", "go.mod"))
	require.NoError(t, err)
	expected, err := filepath.EvalSymlinks(manifestFile)
	require.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestFindWorkspaceManifestRespectsDepthLimit(t *testing.T) {
	tmpDir := t.TempDir()
	// Place the manifest 2 levels above the submodule
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "govendor.toml"), []byte("test"), 0o644))
	root := filepath.Join(tmpDir, "root")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "a"), 0o755))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	defer func() {
		require.NoError(t, os.Chdir(origDir))
	}()

	// a/go.mod → path component count = 1 → maxDepth = 1
	// Manifest is 2 levels up (root → tmpDir), which exceeds the depth limit
	result, err := vendor.FindWorkspaceManifest("a/go.mod")
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestFindWorkspaceManifestStripsFilename(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "theme"), 0o755))

	manifestFile := filepath.Join(tmpDir, "govendor.toml")
	require.NoError(t, os.WriteFile(manifestFile, []byte("test"), 0o644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer func() {
		require.NoError(t, os.Chdir(origDir))
	}()

	// go.mod suffix is stripped before computing depth — result should be the same
	// whether the caller passes a directory or a path with a filename
	withFile, err := vendor.FindWorkspaceManifest("theme/go.mod")
	require.NoError(t, err)

	withDir, err := vendor.FindWorkspaceManifest("theme")
	require.NoError(t, err)

	expected, err := filepath.EvalSymlinks(manifestFile)
	require.NoError(t, err)
	assert.Equal(t, expected, withFile)
	assert.Equal(t, expected, withDir)
}

package discover_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/purpleclay/go-overlay/cmd/govendor/internal/discover"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const workspaceManifest = "schema = 4\n\n[workspace]\ngo = \"1.25\"\nmodules = [\"./api\"]\n\n[mod]\n"

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
}

func TestTargetsModuleCarriesItsExistingManifest(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"go.mod":        "module example.com/app\n\ngo 1.25\n",
		"govendor.toml": "schema = 4\n\n[mod]\n",
	})

	targets, err := discover.Targets([]string{filepath.Join(dir, "go.mod")})
	require.NoError(t, err)

	require.Len(t, targets, 1)
	assert.Equal(t, discover.Module, targets[0].Kind)
	assert.Equal(t, filepath.Join(dir, "go.mod"), targets[0].Path)
	assert.Equal(t, "example.com/app", targets[0].Mod.ModulePath)
	assert.Equal(t, "schema = 4\n\n[mod]\n", string(targets[0].Existing))
	assert.NoError(t, targets[0].Err)
}

func TestTargetsModuleWithoutAManifestHasNoExisting(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"go.mod": "module example.com/app\n\ngo 1.25\n"})

	targets, err := discover.Targets([]string{dir})
	require.NoError(t, err)

	require.Len(t, targets, 1)
	assert.Nil(t, targets[0].Existing)
}

func TestTargetsWorkspaceFromGoWork(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"go.work":    "go 1.25\n\nuse ./api\n",
		"api/go.mod": "module example.com/api\n\ngo 1.25\n",
	})

	targets, err := discover.Targets([]string{dir})
	require.NoError(t, err)

	require.Len(t, targets, 1)
	assert.Equal(t, discover.Workspace, targets[0].Kind)
	assert.Equal(t, filepath.Join(dir, "go.work"), targets[0].Path)
	assert.Equal(t, []string{"./api"}, targets[0].Work.Modules)
}

func TestTargetsWorkspaceRebuiltFromManifest(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"govendor.toml": workspaceManifest,
		"api/go.mod":    "module example.com/api\n\ngo 1.25\n",
	})

	targets, err := discover.Targets([]string{dir})
	require.NoError(t, err)

	require.Len(t, targets, 1)
	assert.Equal(t, discover.Workspace, targets[0].Kind)
	assert.Equal(t, filepath.Join(dir, "go.work"), targets[0].Path)
	assert.Equal(t, []string{"api"}, targets[0].Work.Modules)
	assert.Equal(t, workspaceManifest, string(targets[0].Existing))
}

func TestTargetsReportsAMissingGoModAsATarget(t *testing.T) {
	dir := t.TempDir()

	targets, err := discover.Targets([]string{dir})
	require.NoError(t, err)

	require.Len(t, targets, 1)
	assert.Equal(t, filepath.Join(dir, "go.mod"), targets[0].Path)
	assert.EqualError(t, targets[0].Err, "go.mod does not exist, check path")
}

func TestTargetsRecursiveFindsEveryModule(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"go.work":        "go 1.25\n\nuse ./api\n",
		"api/go.mod":     "module example.com/api\n\ngo 1.25\n",
		"cmd/cli/go.mod": "module example.com/cli\n\ngo 1.25\n",
	})

	targets, err := discover.Targets([]string{dir}, discover.WithRecursive(0))
	require.NoError(t, err)

	paths := make([]string, 0, len(targets))
	for _, tgt := range targets {
		assert.Equal(t, discover.Module, tgt.Kind, "recursive mode ignores go.work")
		paths = append(paths, tgt.Path)
	}
	assert.ElementsMatch(t, []string{
		filepath.Join(dir, "api", "go.mod"),
		filepath.Join(dir, "cmd", "cli", "go.mod"),
	}, paths)
}

func TestTargetsWorkspaceModeFindsTheEnclosingWorkspace(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"govendor.toml": workspaceManifest,
		"api/go.mod":    "module example.com/api\n\ngo 1.25\n",
	})
	t.Chdir(dir)

	targets, err := discover.Targets([]string{"api/go.mod"}, discover.WithWorkspace())
	require.NoError(t, err)

	require.Len(t, targets, 1)
	assert.Equal(t, discover.Workspace, targets[0].Kind)
	assert.NoError(t, targets[0].Err)
	assert.Equal(t, "go.work", filepath.Base(targets[0].Path))
}

func TestTargetsWorkspaceModeWithoutAManifest(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"api/go.mod": "module example.com/api\n\ngo 1.25\n"})
	t.Chdir(dir)

	targets, err := discover.Targets([]string{"api"}, discover.WithWorkspace())
	require.NoError(t, err)

	require.Len(t, targets, 1)
	assert.ErrorIs(t, targets[0].Err, discover.ErrNoManifest)
	assert.Equal(t, filepath.Join("api", "govendor.toml"), targets[0].Path)
}

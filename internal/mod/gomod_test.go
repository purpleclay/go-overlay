package mod_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/purpleclay/go-overlay/internal/mod"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const goModFile = "go.mod"

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestParseGoModFile(t *testing.T) {
	content := `
module github.com/purpleclay/example/with-deps

go 1.25.4

require github.com/fatih/color v1.18.0

require (
		github.com/mattn/go-colorable v0.1.13 // indirect
		github.com/mattn/go-isatty v0.0.20 // indirect
		golang.org/x/sys v0.25.0 // indirect
)
`
	dir := t.TempDir()
	path := writeFile(t, dir, goModFile, content)

	goMod, err := mod.ParseGoModFile(path)
	require.NoError(t, err)

	assert.Equal(t, dir, goMod.Dir())
	assert.Equal(t, "github.com/purpleclay/example/with-deps", goMod.ModulePath())
	assert.Equal(t, "1.25.4", goMod.GoVersion())
	assert.True(t, goMod.HasDependencies())
	assert.False(t, goMod.HasTools())
	assert.Equal(t, "sha256-rgxUeyQeYlhhzhUV4JLESO1HsjBfOWl58oFqkobyYus=", goMod.Hash())

	reqs := goMod.Requires()
	assert.Len(t, reqs, 4)
	assert.Equal(t, "v1.18.0", reqs["github.com/fatih/color"])
	assert.Equal(t, "v0.1.13", reqs["github.com/mattn/go-colorable"])
	assert.Equal(t, "v0.0.20", reqs["github.com/mattn/go-isatty"])
	assert.Equal(t, "v0.25.0", reqs["golang.org/x/sys"])
}

func TestParseGoModFileReturnsErrorForMissingModuleDirective(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, goModFile, "go 1.25.4\n")

	_, err := mod.ParseGoModFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing a module directive")
}

func TestParseGoModFileNoDeps(t *testing.T) {
	content := `
module github.com/purpleclay/example/no-deps

go 1.25.4
`
	dir := t.TempDir()
	path := writeFile(t, dir, goModFile, content)

	goMod, err := mod.ParseGoModFile(path)
	require.NoError(t, err)
	assert.False(t, goMod.HasDependencies())
}

func TestLocalReplacements(t *testing.T) {
	content := `
module github.com/purpleclay/example/replacements

go 1.25.4

require (
	github.com/purpleclay/example/local v1.0.0
	github.com/purpleclay/example/remote v1.0.0
)

replace github.com/purpleclay/example/local => ./libs/local
replace github.com/purpleclay/example/remote => github.com/fork/remote v1.0.0
`
	dir := t.TempDir()
	path := writeFile(t, dir, goModFile, content)

	goMod, err := mod.ParseGoModFile(path)
	require.NoError(t, err)

	local := goMod.LocalReplacements()
	require.Len(t, local, 1)
	assert.Equal(t, "github.com/purpleclay/example/local", local[0].OldPath)
	assert.Equal(t, "./libs/local", local[0].NewPath)
}

func TestRemoteReplacements(t *testing.T) {
	content := `
module github.com/purpleclay/example/replacements

go 1.25.4

require (
	github.com/purpleclay/example/local v1.0.0
	github.com/purpleclay/example/remote v1.0.0
)

replace github.com/purpleclay/example/local => ./libs/local
replace github.com/purpleclay/example/remote => github.com/fork/remote v1.0.0
`
	dir := t.TempDir()
	path := writeFile(t, dir, goModFile, content)

	goMod, err := mod.ParseGoModFile(path)
	require.NoError(t, err)

	remote := goMod.RemoteReplacements()
	require.Len(t, remote, 1)
	assert.Equal(t, "github.com/purpleclay/example/remote", remote["github.com/fork/remote"].OldPath)
}

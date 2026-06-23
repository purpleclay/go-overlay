package govendor_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/purpleclay/go-overlay/internal/cli/govendor"
	"github.com/purpleclay/x/cli"
	"github.com/stretchr/testify/require"
)

func writeGoMod(t *testing.T, path, goVersion string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte("module example\n\ngo "+goVersion+"\n"), 0o644))
}

func induceDrift(t *testing.T, dir string) {
	t.Helper()
	manifestPath := filepath.Join(dir, "govendor.toml")
	f, err := os.OpenFile(manifestPath, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	defer f.Close()
	_, err = f.WriteString("# drift\n")
	require.NoError(t, err)
}

// TestExecuteExitCodes exercises the real CLI command end to end (cobra flag
// parsing, vendor resolution, and exit-code mapping together) against the
// gofmt / terraform fmt -check convention: 0 success, 1 drift, 2 error.
func TestExecuteExitCodes(t *testing.T) {
	version := cli.VersionInfo{}

	t.Run("0_ManifestGenerated", func(t *testing.T) {
		dir := t.TempDir()
		writeGoMod(t, filepath.Join(dir, "go.mod"), "1.22")

		code, err := govendor.Execute(version, []string{dir})
		require.NoError(t, err)
		require.Equal(t, 0, code)

		code, err = govendor.Execute(version, []string{"--check", dir})
		require.NoError(t, err)
		require.Equal(t, 0, code)
	})

	t.Run("1_DriftDetected", func(t *testing.T) {
		dir := t.TempDir()
		writeGoMod(t, filepath.Join(dir, "go.mod"), "1.22")

		_, err := govendor.Execute(version, []string{dir})
		require.NoError(t, err)

		induceDrift(t, dir)

		code, err := govendor.Execute(version, []string{"--check", dir})
		require.Error(t, err)
		require.Equal(t, 1, code)
	})

	t.Run("1_ManifestMissing", func(t *testing.T) {
		dir := t.TempDir()
		writeGoMod(t, filepath.Join(dir, "go.mod"), "1.22")

		code, err := govendor.Execute(version, []string{"--check", dir})
		require.Error(t, err)
		require.Equal(t, 1, code)
	})

	t.Run("2_BadFlagCombination", func(t *testing.T) {
		code, err := govendor.Execute(version, []string{"--workspace"})
		require.Error(t, err)
		require.Equal(t, 2, code)
	})

	t.Run("2_UnknownFlag", func(t *testing.T) {
		code, err := govendor.Execute(version, []string{"--definitely-not-a-real-flag"})
		require.Error(t, err)
		require.Equal(t, 2, code)
	})

	t.Run("2_UnparsableGoMod", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("this is not valid go.mod content\n"), 0o644))

		code, err := govendor.Execute(version, []string{"--check", dir})
		require.Error(t, err)
		require.Equal(t, 2, code)
	})

	t.Run("2_MixedSeverityReportsMostSevere", func(t *testing.T) {
		driftDir := t.TempDir()
		writeGoMod(t, filepath.Join(driftDir, "go.mod"), "1.22")
		_, err := govendor.Execute(version, []string{driftDir})
		require.NoError(t, err)
		induceDrift(t, driftDir)

		errorDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(errorDir, "go.mod"), []byte("this is not valid go.mod content\n"), 0o644))

		code, err := govendor.Execute(version, []string{"--check", driftDir, errorDir})
		require.Error(t, err)
		require.Equal(t, 2, code)
	})
}

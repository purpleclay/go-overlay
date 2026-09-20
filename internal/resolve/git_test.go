package resolve

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitTrackedFiles(t *testing.T) {
	output := strings.Join([]string{
		"main.go",
		"cmd/cli/root.go",
		"cmd/cli/version.go",
		"internal/server/serve.go",
	}, "\x00") + "\x00"

	fake := &fakeExecutor{
		responses: map[string]string{
			"git ls-files -z": output,
		},
	}

	tests := []struct {
		name string
		dir  string
	}{
		{name: "clean path", dir: "/http-test"},
		{name: "trailing slash", dir: "/http-test/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracked, err := GitTrackedFiles(context.Background(), fake, tt.dir)
			require.NoError(t, err)

			// Regardless of input form, results are keyed by cleaned paths.
			clean := "/http-test"
			require.Len(t, tracked, 9)
			assert.Contains(t, tracked, clean)
			assert.Contains(t, tracked, filepath.Join(clean, "main.go"))
			assert.Contains(t, tracked, filepath.Join(clean, "cmd"))
			assert.Contains(t, tracked, filepath.Join(clean, "cmd/cli"))
			assert.Contains(t, tracked, filepath.Join(clean, "cmd/cli/root.go"))
			assert.Contains(t, tracked, filepath.Join(clean, "cmd/cli/version.go"))
			assert.Contains(t, tracked, filepath.Join(clean, "internal"))
			assert.Contains(t, tracked, filepath.Join(clean, "internal/server"))
			assert.Contains(t, tracked, filepath.Join(clean, "internal/server/serve.go"))
		})
	}
}

func TestGitTrackedFilesHandlesQuotedNames(t *testing.T) {
	dir := t.TempDir()

	names := []string{"café.go", `file"with"quotes.go`, `back\slash.go`}
	for _, n := range names {
		require.NoError(t, os.WriteFile(filepath.Join(dir, n), []byte("package x\n"), 0o644))
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	runGit("init", "-q")
	runGit("add", "-A")

	tracked, err := GitTrackedFiles(context.Background(), OSExecutor{}, dir)
	require.NoError(t, err)

	for _, n := range names {
		assert.Contains(t, tracked, filepath.Join(dir, n), "expected %s to be tracked", n)
	}
}

func TestGitTrackedFilesWrapsNonGitError(t *testing.T) {
	dir := t.TempDir()

	_, err := GitTrackedFiles(context.Background(), OSExecutor{}, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), dir)
	assert.Contains(t, err.Error(), "git-tracked")
	assert.Contains(t, err.Error(), "is not inside a git repository")
}

func TestGitTrackedFilesWrapsNonGitErrorRegardlessOfLocale(t *testing.T) {
	t.Setenv("LANG", "fr_FR.UTF-8")
	t.Setenv("LC_ALL", "fr_FR.UTF-8")

	dir := t.TempDir()

	_, err := GitTrackedFiles(context.Background(), OSExecutor{}, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not inside a git repository")
}

// erroringExecutor always returns the given error, standing in for
// Executor.Run failures unrelated to git's own "not a git repository"
// diagnosis.
type erroringExecutor struct {
	err error
}

func (e erroringExecutor) Run(context.Context, Command) (string, error) {
	return "", e.err
}

func TestGitTrackedFilesDoesNotMisdiagnoseUnrelatedFailures(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name string
		err  error
	}{
		{name: "cancellation", err: context.Canceled},
		{name: "missing git binary", err: &ExecError{Err: errors.New(`exec: "git": executable file not found in $PATH`)}},
		{name: "permission denied", err: &ExecError{Err: errors.New("exit status 128"), Stderr: "fatal: could not open directory: Permission denied"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GitTrackedFiles(context.Background(), erroringExecutor{err: tt.err}, dir)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "is not inside a git repository")
			assert.Contains(t, err.Error(), dir)
		})
	}
}

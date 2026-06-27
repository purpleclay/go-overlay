package vendor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAtomicWriteLeavesNoOrphanedTempFile(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, vendorFile)

		require.NoError(t, atomicWrite(path, []byte("hello")))

		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "hello", string(got))

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, vendorFile, entries[0].Name())
	})

	t.Run("RenameFailure", func(t *testing.T) {
		dir := t.TempDir()
		// A directory at the target path makes the final rename fail.
		path := filepath.Join(dir, vendorFile)
		require.NoError(t, os.Mkdir(path, 0o755))

		require.Error(t, atomicWrite(path, []byte("hello")))

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, entries, 1, "temp file must be cleaned up after a rename failure")
		assert.Equal(t, vendorFile, entries[0].Name())
	})
}

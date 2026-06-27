package vendor

import (
	"fmt"
	"os"
	"path/filepath"
)

// atomicWrite writes data to path by first writing to a temporary file in
// the same directory, fsyncing it, then renaming it over path. The rename
// is the only operation that touches path itself, so an interrupted
// process (Ctrl+C, an OOM-killed CI job) can only ever leave behind an
// orphaned temp file, never a truncated path.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, ".govendor-*.toml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return fmt.Errorf("failed to set temp file permissions: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file to %s: %w", path, err)
	}

	return nil
}

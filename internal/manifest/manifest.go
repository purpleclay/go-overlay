package manifest

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Manifest is implemented by any type that can render itself as a Nix file.
type Manifest interface {
	Filename() string
	String() string
}

// Write writes m to a file under outputDir if set, otherwise prints to w.
// If outputDir does not exist it is created automatically.
func Write(m Manifest, outputDir string, w io.Writer) error {
	if outputDir == "" {
		fmt.Fprint(w, m.String())
		return nil
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	path := filepath.Join(outputDir, m.Filename())
	return os.WriteFile(path, []byte(m.String()), 0o644)
}

package resolve

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/nix-community/go-nix/pkg/nar"
)

// NARHash computes the NAR hash of a directory in SRI format.
func NARHash(dir string) (string, error) {
	h := sha256.New()
	if err := nar.DumpPath(h, dir); err != nil {
		return "", fmt.Errorf("failed to hash directory %s: %w", dir, err)
	}
	return "sha256-" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// NARHashFiltered computes the NAR hash of a directory, including only
// paths that pass the filter function. Used for local modules where
// only git-tracked files should be included.
func NARHashFiltered(dir string, filter nar.SourceFilterFunc) (string, error) {
	h := sha256.New()
	if err := nar.DumpPathFilter(h, dir, filter); err != nil {
		return "", fmt.Errorf("failed to hash directory %s: %w", dir, err)
	}
	return "sha256-" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

package narhash

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nix-community/go-nix/pkg/nar"
)

// Dir computes the NAR hash of a directory in SRI format.
func Dir(dir string) (string, error) {
	h := sha256.New()
	if err := nar.DumpPath(h, dir); err != nil {
		return "", fmt.Errorf("failed to hash directory %s: %w", dir, err)
	}
	return "sha256-" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// DirFiltered computes the NAR hash of a directory, including only
// paths that pass the filter function. Used for local modules where
// only git-tracked files should be included.
func DirFiltered(dir string, filter nar.SourceFilterFunc) (string, error) {
	h := sha256.New()
	if err := nar.DumpPathFilter(h, dir, filter); err != nil {
		return "", fmt.Errorf("failed to hash directory %s: %w", dir, err)
	}
	return "sha256-" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// GitTracked computes the NAR hash of a directory including only
// git-tracked files, excluding macOS .DS_Store files. This is the standard
// filter for local module hashing throughout the resolver.
func GitTracked(dir string, tracked map[string]struct{}) (string, error) {
	return DirFiltered(dir, func(path string, _ nar.NodeType) bool {
		if strings.ToLower(filepath.Base(path)) == ".ds_store" {
			return false
		}
		_, ok := tracked[path]
		return ok
	})
}

package resolve

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nix-community/go-nix/pkg/nar"
)

// Hasher computes the NAR hash of a directory. The interface allows tests to
// inject a counting or deterministic implementation without hitting the real
// filesystem hasher.
//
// HashGitTracked is part of the interface rather than a free function so
// local modules go through the same injectable seam as downloaded ones —
// otherwise half the resolver's hashing is untestable without a real git
// checkout on disk.
type Hasher interface {
	Hash(dir string) (string, error)
	HashGitTracked(dir string, tracked map[string]struct{}) (string, error)
}

// NARHasher is the default Hasher, delegating to the NARHash functions.
type NARHasher struct{}

func (NARHasher) Hash(dir string) (string, error) { return NARHash(dir) }

func (NARHasher) HashGitTracked(dir string, tracked map[string]struct{}) (string, error) {
	return NARHashGitTracked(dir, tracked)
}

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

// NARHashGitTracked computes the NAR hash of a directory including only
// git-tracked files, excluding macOS .DS_Store files. This is the standard
// filter for local module hashing throughout the resolver.
func NARHashGitTracked(dir string, tracked map[string]struct{}) (string, error) {
	return NARHashFiltered(dir, func(path string, _ nar.NodeType) bool {
		if strings.ToLower(filepath.Base(path)) == ".ds_store" {
			return false
		}
		_, ok := tracked[path]
		return ok
	})
}

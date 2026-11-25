package resolve

import "github.com/purpleclay/go-overlay/internal/narhash"

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

// NARHasher is the default Hasher, delegating to the narhash package.
type NARHasher struct{}

func (NARHasher) Hash(dir string) (string, error) { return narhash.Dir(dir) }

func (NARHasher) HashGitTracked(dir string, tracked map[string]struct{}) (string, error) {
	return narhash.GitTracked(dir, tracked)
}

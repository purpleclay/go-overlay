package resolve

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// GitTrackedFiles returns a set of absolute paths for all files tracked by
// git in the given directory, along with all intermediate subdirectories
// between dir and each tracked file. The returned set is suitable for use as
// a file filter when hashing a local module — only git-tracked paths are
// included, ensuring untracked files are excluded from the NAR hash.
func GitTrackedFiles(ctx context.Context, exec Executor, dir string) (map[string]struct{}, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	dir = filepath.Clean(absDir)
	// -z disables git's default path quoting (core.quotePath) and emits
	// NUL-terminated entries instead, so a name containing a non-ASCII byte,
	// control character, quote, or backslash round-trips exactly. Splitting
	// plain `git ls-files` output on newlines would otherwise insert the
	// quoted, escaped form into tracked, which never matches the real path
	// the NAR walker sees, silently dropping the file from the hash.
	out, err := exec.Run(ctx, []string{"git", "ls-files", "-z"}, dir, []string{"LC_ALL=C"})
	if err != nil {
		var execErr *ExecError
		if errors.As(err, &execErr) && strings.Contains(execErr.Stderr, "not a git repository") {
			return nil, fmt.Errorf("local module %s is not inside a git repository; govendor hashes only git-tracked files: %w", dir, err)
		}
		return nil, fmt.Errorf("failed to list git-tracked files for local module %s: %w", dir, err)
	}

	tracked := make(map[string]struct{})
	tracked[dir] = struct{}{}

	for rel := range strings.SplitSeq(out, "\x00") {
		if rel == "" {
			continue
		}
		abs := filepath.Join(dir, rel)
		tracked[abs] = struct{}{}
		for parent := filepath.Dir(abs); parent != dir; {
			tracked[parent] = struct{}{}
			next := filepath.Dir(parent)
			if next == parent {
				break
			}
			parent = next
		}
	}

	return tracked, nil
}

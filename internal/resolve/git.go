package resolve

import (
	"path/filepath"
	"strings"
)

// GitTrackedFiles returns a set of absolute paths for all files tracked by
// git in the given directory, along with all intermediate subdirectories
// between dir and each tracked file. The returned set is suitable for use as
// a file filter when hashing a local module — only git-tracked paths are
// included, ensuring untracked files are excluded from the NAR hash.
func GitTrackedFiles(exec Executor, dir string) (map[string]struct{}, error) {
	dir = filepath.Clean(dir)
	out, err := exec.Run([]string{"git", "ls-files"}, dir, nil)
	if err != nil {
		return nil, err
	}

	tracked := make(map[string]struct{})
	tracked[dir] = struct{}{}

	for rel := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
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

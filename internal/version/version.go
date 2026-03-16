package version

import (
	"fmt"
	"strings"
)

// Latest returns the last element from a sorted list of versions, which is
// the most recent. Returns an error if the slice is empty.
func Latest(versions []string, module string) (string, error) {
	if len(versions) == 0 {
		return "", fmt.Errorf("no versions found for module %s", module)
	}
	return versions[len(versions)-1], nil
}

// TrimGlob reports whether pattern ends with a wildcard (*) and returns the
// prefix with the wildcard stripped. If pattern is not a glob, it returns the
// original string and false.
func TrimGlob(pattern string) (string, bool) {
	return strings.CutSuffix(pattern, "*")
}

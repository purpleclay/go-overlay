package version

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

// Latest returns the last element from a sorted list of versions, which is
// the most recent. Returns an error if the slice is empty.
func Latest(versions []string, module string) (string, error) {
	if len(versions) == 0 {
		return "", fmt.Errorf("no versions found for module %s", module)
	}
	return versions[len(versions)-1], nil
}

// ExcludePrerelease returns versions with a semver prerelease component
// (e.g. v1.1.0-rc1) filtered out.
func ExcludePrerelease(versions []string) []string {
	result := make([]string, 0, len(versions))
	for _, v := range versions {
		if semver.Prerelease(v) == "" {
			result = append(result, v)
		}
	}
	return result
}

// TrimGlob reports whether pattern ends with a wildcard (*) and returns the
// prefix with the wildcard stripped. If pattern is not a glob, it returns the
// original string and false.
func TrimGlob(pattern string) (string, bool) {
	return strings.CutSuffix(pattern, "*")
}

package resolve

import (
	"sort"
	"strings"
)

// ParsePackagesByModule parses the tab-separated output of `go list` into a
// map of module path to imported package paths. Lines without a tab are
// skipped.
func ParsePackagesByModule(out string) map[string][]string {
	pkgsByMod := make(map[string][]string)
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		modPath, pkgPath, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		pkgsByMod[modPath] = append(pkgsByMod[modPath], pkgPath)
	}
	return pkgsByMod
}

// MergePackages combines two package lists, deduplicating and sorting.
func MergePackages(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	for _, p := range a {
		seen[p] = true
	}
	for _, p := range b {
		seen[p] = true
	}

	result := make([]string, 0, len(seen))
	for p := range seen {
		result = append(result, p)
	}
	sort.Strings(result)
	return result
}

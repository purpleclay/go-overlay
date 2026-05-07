package vendor

import (
	"fmt"
	"maps"
	"slices"

	"github.com/purpleclay/go-overlay/internal/mod"
)

// IsDrifted reports whether the dependency source's current state differs
// from what is recorded in the existing manifest.
func IsDrifted(src dependencySource, existing *Manifest) (bool, error) {
	switch s := src.(type) {
	case *mod.GoModFile:
		return requiresDrifted(s.Requires, existing.Mod) ||
			replacementsDrifted(s.Replacements, existing.Mod) ||
			excludesDrifted(s.Excludes, existing.Exclude), nil
	case *mod.GoWorkFile:
		members, err := s.ParseMembers()
		if err != nil {
			return false, err
		}
		requires := make(map[string]string)
		replacements := make(map[string]mod.Replacement)
		excludes := make(map[string][]string)
		for _, m := range members {
			maps.Copy(requires, m.Requires)
			maps.Copy(replacements, m.Replacements)
			for path, versions := range m.Excludes {
				excludes[path] = append(excludes[path], versions...)
			}
		}
		for path, versions := range excludes {
			slices.Sort(versions)
			excludes[path] = slices.Compact(versions)
		}
		// Workspace-level replacements take precedence over member-level ones.
		maps.Copy(replacements, s.Replacements)
		return requiresDrifted(requires, existing.Mod) ||
			replacementsDrifted(replacements, existing.Mod) ||
			excludesDrifted(excludes, existing.Exclude), nil
	default:
		return false, fmt.Errorf("unsupported dependency source: %T", src)
	}
}

func requiresDrifted(requires map[string]string, mods map[string]mod.ModuleConfig) bool {
	if len(requires) != len(mods) {
		return true
	}
	for path, version := range requires {
		if m, ok := mods[path]; !ok || m.Version != version {
			return true
		}
	}
	return false
}

// replacementsDrifted returns true if any replace directive has been added, removed, or changed.
func replacementsDrifted(replacements map[string]mod.Replacement, mods map[string]mod.ModuleConfig) bool {
	for oldPath, repl := range replacements {
		m, ok := mods[oldPath]
		if !ok {
			return true
		}
		if repl.IsLocal && m.Local != repl.LocalPath {
			return true
		}
		if !repl.IsLocal && m.ReplacedPath != repl.NewPath {
			return true
		}
	}
	// Detect removed replacements: any mod recorded as replaced must still
	// have a corresponding replace directive in the current go.mod.
	for path, m := range mods {
		if m.Local != "" || m.ReplacedPath != "" {
			if _, ok := replacements[path]; !ok {
				return true
			}
		}
	}
	return false
}

func excludesDrifted(excludes map[string][]string, recorded map[string][]string) bool {
	if len(excludes) != len(recorded) {
		return true
	}
	for path, versions := range excludes {
		if stored, ok := recorded[path]; !ok || !slices.Equal(versions, stored) {
			return true
		}
	}
	return false
}

package mod

import (
	"fmt"
	"strings"
)

func ValidatePlatforms(platforms []string) error {
	if len(platforms) == 0 {
		return nil
	}

	supported, err := supportedPlatforms()
	if err != nil {
		return fmt.Errorf("failed to get supported platforms: %w", err)
	}

	var invalid []string
	for _, p := range platforms {
		if !supported[p] {
			invalid = append(invalid, p)
		}
	}

	if len(invalid) > 0 {
		return fmt.Errorf("unsupported platform(s): %s", strings.Join(invalid, ", "))
	}

	return nil
}

func supportedPlatforms() (map[string]bool, error) {
	out, err := exec([]string{"go", "tool", "dist", "list"}, ".")
	if err != nil {
		return nil, err
	}

	platforms := make(map[string]bool)
	for line := range strings.SplitSeq(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			platforms[line] = true
		}
	}

	return platforms, nil
}

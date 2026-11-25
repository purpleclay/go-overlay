package modproxy

import (
	"os"
	"path/filepath"

	"github.com/google/licensecheck"
)

// licenseFilenames are the license file names checked at the root of a
// downloaded module source. All tools currently managed by this overlay use
// a plain LICENSE file, so we only scan for that.
var licenseFilenames = []string{
	"LICENSE",
}

// licenseMatchThreshold is the minimum coverage percentage required before a
// license match is considered confident enough to record.
const licenseMatchThreshold = 75.0

// deprecatedSPDX maps the deprecated SPDX identifiers that licensecheck v0.3.1
// may return to their canonical non-deprecated equivalents. The deprecated forms
// are ambiguous (they predate the -only/-or-later distinction), so we normalise
// them to -only, which matches the intent of a plain "Version N" license text
// that contains no "or later version" grant.
var deprecatedSPDX = map[string]string{
	"AGPL-1.0": "AGPL-1.0-only",
	"AGPL-3.0": "AGPL-3.0-only",
	"GPL-1.0":  "GPL-1.0-only",
	"GPL-2.0":  "GPL-2.0-only",
	"GPL-3.0":  "GPL-3.0-only",
	"LGPL-2.0": "LGPL-2.0-only",
	"LGPL-2.1": "LGPL-2.1-only",
	"LGPL-3.0": "LGPL-3.0-only",
}

// detectLicense returns the SPDX identifier of the license found in srcDir,
// or "" if no license file is found or its content doesn't confidently match
// a known license.
func detectLicense(srcDir string) (string, error) {
	for _, name := range licenseFilenames {
		content, err := os.ReadFile(filepath.Join(srcDir, name))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}

		cov := licensecheck.Scan(content)
		if len(cov.Match) == 0 || cov.Percent < licenseMatchThreshold {
			return "", nil
		}

		best := cov.Match[0]
		for _, m := range cov.Match[1:] {
			if (m.End - m.Start) > (best.End - best.Start) {
				best = m
			}
		}

		id := best.ID
		if canonical, ok := deprecatedSPDX[id]; ok {
			id = canonical
		}
		return id, nil
	}

	return "", nil
}

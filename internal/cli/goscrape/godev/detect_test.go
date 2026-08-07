package godev

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectPatchVersion(t *testing.T) {
	fd, err := os.ReadFile("testdata/index-20260215.html")
	require.NoError(t, err)

	version, err := detectVersion(string(fd), "1.25", false)
	require.NoError(t, err)
	assert.Equal(t, "1.25.7", version)
}

func TestDetectPrefixExcludesPrereleaseByDefault(t *testing.T) {
	fd, err := os.ReadFile("testdata/index-20260806.html")
	require.NoError(t, err)

	// 1.27 exists only as release candidates (1.27rc1, 1.27rc2) at the time
	// this fixture was captured, with no stable 1.27.0 yet. Without
	// --include-prerelease, detecting against that prefix must not silently
	// return a release candidate.
	_, err = detectVersion(string(fd), "1.27", false)
	require.Error(t, err)
}

func TestDetectPrefixIncludesPrereleaseWhenRequested(t *testing.T) {
	fd, err := os.ReadFile("testdata/index-20260806.html")
	require.NoError(t, err)

	version, err := detectVersion(string(fd), "1.27", true)
	require.NoError(t, err)
	assert.Equal(t, "1.27rc2", version)
}

func TestDetectPrefixStillPrefersStableWhenBothExist(t *testing.T) {
	fd, err := os.ReadFile("testdata/index-20260806.html")
	require.NoError(t, err)

	version, err := detectVersion(string(fd), "1.26", false)
	require.NoError(t, err)
	assert.Equal(t, "1.26.5", version)
}

func TestLatestFromPageIncludesPrerelease(t *testing.T) {
	fd, err := os.ReadFile("testdata/index-20260215.html")
	require.NoError(t, err)

	// Scanning the page must use semantic ordering, not the lexicographic sort
	// used for listing: 1.26.0 outranks every 1.25 patch and 1.26 candidate.
	version, err := latestFromPage(string(fd), "")
	require.NoError(t, err)
	assert.Equal(t, "1.26.0", version)
}

func TestLatestFromPagePrereleaseWins(t *testing.T) {
	// A new minor's release candidate outranks the latest stable. This is the
	// core motivation: the stable VERSION endpoint would only report 1.26.4.
	const page = `<a class="download" href="/dl/go1.25.10.linux-amd64.tar.gz">go1.25.10</a>
<a class="download" href="/dl/go1.26.4.linux-amd64.tar.gz">go1.26.4</a>
<a class="download" href="/dl/go1.27rc1.linux-amd64.tar.gz">go1.27rc1</a>
`

	version, err := latestFromPage(page, "")
	require.NoError(t, err)
	assert.Equal(t, "1.27rc1", version)
}

func TestListVersions(t *testing.T) {
	fd, err := os.ReadFile("testdata/index-20260215.html")
	require.NoError(t, err)

	versions, err := listVersions(string(fd), "")
	require.NoError(t, err)
	assert.NotEmpty(t, versions)
}

func TestListVersionsDistinguishesBetaFromStable(t *testing.T) {
	fd, err := os.ReadFile("testdata/index-20260806.html")
	require.NoError(t, err)

	// go1.19beta1 predates the current stable go1.19 release line. Both must
	// be listed as their own distinct versions rather than merging into a
	// single "1.19" entry.
	versions, err := listVersions(string(fd), "1.19")
	require.NoError(t, err)

	assert.Contains(t, versions, "1.19beta1")
	assert.Contains(t, versions, "1.19")
}

func TestListVersionsWithPrefix1_25(t *testing.T) {
	fd, err := os.ReadFile("testdata/index-20260215.html")
	require.NoError(t, err)

	versions, err := listVersions(string(fd), "1.25")
	require.NoError(t, err)

	expected := []string{
		"1.25.0",
		"1.25.1",
		"1.25.2",
		"1.25.3",
		"1.25.4",
		"1.25.5",
		"1.25.6",
		"1.25.7",
		"1.25rc1",
		"1.25rc2",
		"1.25rc3",
	}
	assert.Equal(t, expected, versions)
}

package scrape

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoVersionCombinator(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Full",
			input:    "go1.21.4",
			expected: "1.21.4",
		},
		{
			name:     "MinorOnlyNoPrefix",
			input:    "go1.22",
			expected: "1.22",
		},
		{
			name:     "ReleaseCandidate",
			input:    "go1.25rc1",
			expected: "1.25rc1",
		},
		{
			name:     "Beta",
			input:    "go1.19beta1",
			expected: "1.19beta1",
		},
		{
			name:     "BetaWithDownloadFilename",
			input:    "go1.19beta1.linux-amd64.tar.gz",
			expected: "1.19beta1",
		},
		{
			// Regression guard: AIX is a real, currently-supported target
			// whose name starts with 'a'. Any fix for beta parsing must not
			// widen the character class in a way that eats into it.
			name:     "PatchVersionWithAIXTarget",
			input:    "go1.21.4.aix-ppc64.tar.gz",
			expected: "1.21.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, actual, err := GoVersion()(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestGoVersionCombinatorWithInvalidVersion(t *testing.T) {
	_, _, err := GoVersion()("not-a-version")
	assert.Error(t, err)
}

func TestHrefCombinator(t *testing.T) {
	html := `<div>
<a class="download" href="/dl/go1.21.4.linux-amd64.tar.gz">
Download
</a>
</div>`

	_, result, err := Href("go1.21.4")(html)
	require.NoError(t, err)
	require.Equal(t, "/dl/go1.21.4.linux-amd64.tar.gz", result)
}

func TestHrefCombinatorNormalization(t *testing.T) {
	html := `<div>
<a class="download" href="/dl/go1.21.4.linux-amd64.tar.gz">
Download
</a>
</div>`

	_, result, err := Href("1.21.4")(html)
	require.NoError(t, err)
	require.Equal(t, "/dl/go1.21.4.linux-amd64.tar.gz", result)
}

func TestHrefCombinatorMatchesBetaExactly(t *testing.T) {
	html := `<div>
<a class="download" href="/dl/go1.19beta1.linux-amd64.tar.gz">
Download
</a>
</div>`

	_, result, err := Href("1.19beta1")(html)
	require.NoError(t, err)
	require.Equal(t, "/dl/go1.19beta1.linux-amd64.tar.gz", result)
}

func TestHrefCombinatorDoesNotMatchBetaTenWhenRequestingBetaOne(t *testing.T) {
	// beta1 is a string-prefix of beta10, so a naive prefix match on
	// "go1.19beta1" would incorrectly match the beta10 entry if it appears
	// first on the page.
	html := `<div>
<a class="download" href="/dl/go1.19beta10.linux-amd64.tar.gz">
Download
</a>
<a class="download" href="/dl/go1.19beta1.linux-amd64.tar.gz">
Download
</a>
</div>`

	_, result, err := Href("1.19beta1")(html)
	require.NoError(t, err)
	require.Equal(t, "/dl/go1.19beta1.linux-amd64.tar.gz", result)
}

func TestHrefCombinatorDoesNotMatchRcTenWhenRequestingRcOne(t *testing.T) {
	html := `<div>
<a class="download" href="/dl/go1.25rc10.linux-amd64.tar.gz">
Download
</a>
<a class="download" href="/dl/go1.25rc1.linux-amd64.tar.gz">
Download
</a>
</div>`

	_, result, err := Href("1.25rc1")(html)
	require.NoError(t, err)
	require.Equal(t, "/dl/go1.25rc1.linux-amd64.tar.gz", result)
}

func TestHrefCombinatorVersionMissing(t *testing.T) {
	html := `<div>
<a class="download" href="/dl/go1.21.4.linux-amd64.tar.gz">
Download
</a>
</div>`

	_, _, err := Href("go9.99.99")(html)
	assert.Error(t, err)
}

func TestHrefCombinatorEmptyVersionFromPage(t *testing.T) {
	fd, err := os.ReadFile("testdata/index-20260215.html")
	require.NoError(t, err)

	_, result, err := Href("")(string(fd))
	require.NoError(t, err)
	require.Equal(t, "/dl/go1.26.0.src.tar.gz", result)
}

func TestHrefCombinatorEmptyVersion(t *testing.T) {
	html := `<div>
<a class="download" href="/dl/go1.21.4.linux-amd64.tar.gz">
Download
</a>
</div>`

	_, result, err := Href("")(html)
	require.NoError(t, err)
	require.Equal(t, "/dl/go1.21.4.linux-amd64.tar.gz", result)
}

func TestHrefCombinatorEmptyVersionMultiple(t *testing.T) {
	html := `<div>
<a class="download" href="/dl/go1.21.4.linux-amd64.tar.gz">
Download
</a>
<a class="download" href="/dl/go1.22.0.linux-amd64.tar.gz">
Download
</a>
</div>`

	rem, first, err := Href("")(html)
	require.NoError(t, err)
	require.Equal(t, "/dl/go1.21.4.linux-amd64.tar.gz", first)

	_, second, err := Href("")(rem)
	require.NoError(t, err)
	require.Equal(t, "/dl/go1.22.0.linux-amd64.tar.gz", second)
}

func TestTargetCombinator(t *testing.T) {
	tableRow := `  <td>Archive</td>
  <td>macOS</td>
  <td>ARM64</td>
  <td>68MB</td>
  <td><tt>047bfce4fbd0da6426bd30cd19716b35a466b1c15a45525ce65b9824acb33285</tt></td>
`

	_, result, err := Target()(tableRow)
	require.NoError(t, err)

	assert.Len(t, result, 5)
	assert.Equal(t, "Archive", result[0])
	assert.Equal(t, "macOS", result[1])
	assert.Equal(t, "ARM64", result[2])
	assert.Equal(t, "68MB", result[3])
	assert.Equal(t, "047bfce4fbd0da6426bd30cd19716b35a466b1c15a45525ce65b9824acb33285", result[4])
}

func TestSeekDownloadSection(t *testing.T) {
	downloadSection := `<div class="toggle" id="go1.21.4">
		<div class="collapsed">
			<h3 class="toggleButton" title="Click to show downloads for this version">
    <span>go1.21.4</span>
    <img class="toggleButton-img" src="/images/icons/arrow-down.svg" width="18" height="18" aria-hidden="true" />
    <img class="toggleButton-img toggleButton-img-dark" src="/images/icons/arrow-down-dark.svg" width="18" height="18" aria-hidden="true" />
    </h3>
		</div>`

	result, _, err := SeekDownloadSection("1.21.4")(downloadSection)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(result, `id="go1.21.4"`))
}

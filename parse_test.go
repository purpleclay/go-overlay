package main

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, actual, err := goVersion()(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestGoVersionCombinatorWithInvalidVersion(t *testing.T) {
	_, _, err := goVersion()("not-a-version")
	assert.Error(t, err)
}

func TestHrefCombinator(t *testing.T) {
	html := `<div>
<a class="download" href="/dl/go1.21.4.linux-amd64.tar.gz">
Download
</a>
</div>`

	_, result, err := href("go1.21.4")(html)
	require.NoError(t, err)
	require.Equal(t, "/dl/go1.21.4.linux-amd64.tar.gz", result)
}

func TestHrefCombinatorNormalization(t *testing.T) {
	html := `<div>
<a class="download" href="/dl/go1.21.4.linux-amd64.tar.gz">
Download
</a>
</div>`

	_, result, err := href("1.21.4")(html)
	require.NoError(t, err)
	require.Equal(t, "/dl/go1.21.4.linux-amd64.tar.gz", result)
}

func TestHrefCombinatorVersionMissing(t *testing.T) {
	html := `<div>
<a class="download" href="/dl/go1.21.4.linux-amd64.tar.gz">
Download
</a>
</div>`

	_, _, err := href("go9.99.99")(html)
	assert.Error(t, err)
}

func TestTargetCombinator(t *testing.T) {
	tableRow := `  <td>Archive</td>
  <td>macOS</td>
  <td>ARM64</td>
  <td>68MB</td>
  <td><tt>047bfce4fbd0da6426bd30cd19716b35a466b1c15a45525ce65b9824acb33285</tt></td>
`

	_, result, err := target()(tableRow)
	require.NoError(t, err)

	assert.Len(t, result, 5)
	assert.Equal(t, "Archive", result[0])
	assert.Equal(t, "macOS", result[1])
	assert.Equal(t, "ARM64", result[2])
	assert.Equal(t, "68MB", result[3])
	assert.Equal(t, "047bfce4fbd0da6426bd30cd19716b35a466b1c15a45525ce65b9824acb33285", result[4])
}

func TestSeekDownloadSection(t *testing.T) {
	fd, err := os.ReadFile("testdata/index-20251207.html")
	require.NoError(t, err)

	result, _, err := seekDownloadSection("1.21.4")(string(fd))
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(result, `id="go1.21.4"`))
}

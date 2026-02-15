package goscrape

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListVersions(t *testing.T) {
	fd, err := os.ReadFile("testdata/index-20260215.html")
	require.NoError(t, err)

	versions, err := listVersions(string(fd), "")
	require.NoError(t, err)
	assert.NotEmpty(t, versions)
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
	}
	assert.Equal(t, expected, versions)
}

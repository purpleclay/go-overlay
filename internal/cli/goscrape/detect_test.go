package goscrape

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectPatchVersion(t *testing.T) {
	fd, err := os.ReadFile("testdata/index-20260215.html")
	require.NoError(t, err)

	version, err := detectVersion(string(fd), "1.25")
	require.NoError(t, err)
	assert.Equal(t, "1.25.7", version)
}

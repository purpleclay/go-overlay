package goscrape

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gotest.tools/v3/golden"
)

func TestScrape(t *testing.T) {
	fd, err := os.ReadFile("testdata/index-20260215.html")
	require.NoError(t, err)

	// Use a fixed date for reproducible tests
	fixedDate := time.Date(2025, 12, 8, 0, 0, 0, 0, time.UTC)

	s, err := parse(string(fd), "go1.21.6", fixedDate)
	require.NoError(t, err)

	manifest := s.String()
	golden.Assert(t, manifest, "go1.21.6.nix.golden")
}

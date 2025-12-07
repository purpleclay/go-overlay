package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	fd, _ := os.ReadFile("testdata/index-20251207.html")

	s, err := parse(string(fd), "go1.21.6")
	require.NoError(t, err)
	assert.Equal(t, "go1.21.6", s.Version)
	assert.Equal(t, "31d6ecca09010ab351e51343a5af81d678902061fee871f912bdd5ef4d778850", s.Targets["x86_64-darwin"].SHA256)
	assert.Equal(t, "https://go.dev/dl/go1.21.6.darwin-amd64.tar.gz", s.Targets["x86_64-darwin"].URL)
	assert.Equal(t, "e2e8aa88e1b5170a0d495d7d9c766af2b2b6c6925a8f8956d834ad6b4cacbd9a", s.Targets["aarch64-linux"].SHA256)
	assert.Equal(t, "https://go.dev/dl/go1.21.6.linux-arm64.tar.gz", s.Targets["aarch64-linux"].URL)
	assert.Equal(t, "92894d0f732d3379bc414ffdd617eaadad47e1d72610e10d69a1156db03fc052", s.Targets["s390x-linux"].SHA256)
	assert.Equal(t, "https://go.dev/dl/go1.21.6.linux-s390x.tar.gz", s.Targets["s390x-linux"].URL)
	assert.Equal(t, "a35f3d529bb86a41709e659597670284c9f78c9f3928eebc78dd50a2f514bfdf", s.Targets["aarch64-freebsd"].SHA256)
	assert.Equal(t, "https://go.dev/dl/go1.21.6.freebsd-arm64.tar.gz", s.Targets["aarch64-freebsd"].URL)
}

func TestParseVersion(t *testing.T) {
	fd, _ := os.ReadFile("testdata/index-20251207.html")

	ver, err := parseVersion(string(fd), "go1.20")
	require.NoError(t, err)
	assert.Equal(t, "go1.20.14", ver)
}

package goscrapeproxy

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/v3/golden"
)

func TestGenerateManifest(t *testing.T) {
	m, err := generateManifest("golang.org/x/vuln", "v1.1.4", []string{"cmd/govulncheck"})
	require.NoError(t, err)

	golden.Assert(t, m.String(), "v1.1.4.nix.golden")
}

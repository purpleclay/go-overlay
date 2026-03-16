package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLatest(t *testing.T) {
	versions := []string{"v0.1.0", "v0.2.0", "v1.0.0", "v1.0.1", "v1.1.0", "v1.1.1"}

	got, err := Latest(versions, "golang.org/x/vuln")
	require.NoError(t, err)
	assert.Equal(t, "v1.1.1", got)
}

func TestLatestNoVersions(t *testing.T) {
	_, err := Latest([]string{}, "golang.org/x/vuln")
	require.EqualError(t, err, "no versions found for module golang.org/x/vuln")
}

func TestTrimGlob(t *testing.T) {
	prefix, ok := TrimGlob("v1.1*")
	assert.True(t, ok)
	assert.Equal(t, "v1.1", prefix)
}

func TestTrimGlobNotAGlob(t *testing.T) {
	prefix, ok := TrimGlob("v1.1.0")
	assert.False(t, ok)
	assert.Equal(t, "v1.1.0", prefix)
}

package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// proxyListResponse mirrors realistic output from the Go module proxy /@v/list
// endpoint: versions are returned in an arbitrary, unsorted order. Pseudo-
// versions (e.g. v0.0.0-20220614031533-00a90b3619af) are valid semver and are
// included in the response. Non-semver strings (e.g. "latest") are not.
const proxyListResponse = `v1.0.2
v1.0.0
v1.1.1
v1.0.3
v1.1.3
v1.1.2
v1.0.4
v0.1.0
v1.1.4
v1.0.1
v1.1.0
v0.2.0
v0.0.0-20220614031533-00a90b3619af
latest
`

func proxyServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(func() {
		srv.Close()
		baseURL = defaultBaseURL
	})
	baseURL = srv.URL
	return srv
}

func TestListVersionsReturnsSorted(t *testing.T) {
	proxyServer(t, proxyListResponse)

	versions, err := ListVersions("golang.org/x/vuln", "")
	require.NoError(t, err)

	expected := []string{
		"v0.0.0-20220614031533-00a90b3619af",
		"v0.1.0", "v0.2.0",
		"v1.0.0", "v1.0.1", "v1.0.2", "v1.0.3", "v1.0.4",
		"v1.1.0", "v1.1.1", "v1.1.2", "v1.1.3", "v1.1.4",
	}
	assert.Equal(t, expected, versions)
}

func TestListVersionsWithPrefix(t *testing.T) {
	proxyServer(t, proxyListResponse)

	versions, err := ListVersions("golang.org/x/vuln", "1.0")
	require.NoError(t, err)

	expected := []string{"v1.0.0", "v1.0.1", "v1.0.2", "v1.0.3", "v1.0.4"}
	assert.Equal(t, expected, versions)
}

func TestListVersionsStripsNonSemver(t *testing.T) {
	proxyServer(t, proxyListResponse)

	versions, err := ListVersions("golang.org/x/vuln", "")
	require.NoError(t, err)

	for _, v := range versions {
		assert.NotEqual(t, "latest", v, "non-semver string should be excluded")
	}
}

func TestListVersionsEmpty(t *testing.T) {
	proxyServer(t, "")

	versions, err := ListVersions("golang.org/x/vuln", "")
	require.NoError(t, err)
	assert.Empty(t, versions)
}

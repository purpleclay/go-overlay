package resolve

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDownloadOutputReturnsErrorForFailedDownload(t *testing.T) {
	input := `
{
    "Path": "github.com/BurntSushi/toml",
    "Version": "v1.6.0",
    "Error": "dial tcp: lookup proxy.golang.org: no such host"
}`
	_, err := ParseDownloadOutput(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github.com/BurntSushi/toml")
	assert.Contains(t, err.Error(), "v1.6.0")
	assert.Contains(t, err.Error(), "no such host")
}

func TestParseDownloadOutput(t *testing.T) {
	input := `
{
    "Path": "github.com/BurntSushi/toml",
    "Version": "v1.6.0",
    "Info": "/root/go/pkg/mod/cache/download/github.com/!burnt!sushi/toml/@v/v1.6.0.info",
    "GoMod": "/root/go/pkg/mod/cache/download/github.com/!burnt!sushi/toml/@v/v1.6.0.mod",
    "Zip": "/root/go/pkg/mod/cache/download/github.com/!burnt!sushi/toml/@v/v1.6.0.zip",
    "Dir": "/root/go/pkg/mod/github.com/!burnt!sushi/toml@v1.6.0",
    "Sum": "h1:dRaEfpa2VI55EwlIW72hMRHdWouJeRF7TPYhI+AUQjk=",
    "GoModSum": "h1:ukJfTF/6rtPPRCnwkur4qwRxa8vTRFBF0uk2lLoLwho="
}
{
    "Path": "github.com/charlievieth/fastwalk",
    "Version": "v1.0.14",
    "Info": "/root/go/pkg/mod/cache/download/github.com/charlievieth/fastwalk/@v/v1.0.14.info",
    "GoMod": "/root/go/pkg/mod/cache/download/github.com/charlievieth/fastwalk/@v/v1.0.14.mod",
    "Zip": "/root/go/pkg/mod/cache/download/github.com/charlievieth/fastwalk/@v/v1.0.14.zip",
    "Dir": "/root/go/pkg/mod/github.com/charlievieth/fastwalk@v1.0.14",
    "Sum": "h1:3Eh5uaFGwHZd8EGwTjJnSpBkfwfsak9h6ICgnWlhAyg=",
    "GoModSum": "h1:diVcUreiU1aQ4/Wu3NbxxH4/KYdKpLDojrQ1Bb2KgNY="
}`
	downloads, err := ParseDownloadOutput(input)
	require.NoError(t, err)
	require.Len(t, downloads, 2)

	assert.Equal(t, "github.com/BurntSushi/toml", downloads[0].Path)
	assert.Equal(t, "v1.6.0", downloads[0].Version)
	assert.Equal(t, "/root/go/pkg/mod/github.com/!burnt!sushi/toml@v1.6.0", downloads[0].Dir)
	assert.Equal(t, "/root/go/pkg/mod/cache/download/github.com/!burnt!sushi/toml/@v/v1.6.0.mod", downloads[0].GoMod)

	assert.Equal(t, "github.com/charlievieth/fastwalk", downloads[1].Path)
	assert.Equal(t, "v1.0.14", downloads[1].Version)
	assert.Equal(t, "/root/go/pkg/mod/github.com/charlievieth/fastwalk@v1.0.14", downloads[1].Dir)
	assert.Equal(t, "/root/go/pkg/mod/cache/download/github.com/charlievieth/fastwalk/@v/v1.0.14.mod", downloads[1].GoMod)
}

package resolve

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/purpleclay/go-overlay/internal/mod"
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

func TestDownloadFutureWaitReturnsDownloads(t *testing.T) {
	exec := &fakeExecutor{responses: map[string]string{
		"go mod": `{"Path":"example.com/foo","Version":"v1.0.0","Dir":"/cache/foo"}`,
	}}
	r := New(exec)

	f := r.startDownload(context.Background(), "/repo", nil)
	downloads, err := f.wait(context.Background())
	require.NoError(t, err)
	require.Len(t, downloads, 1)
	assert.Equal(t, "example.com/foo", downloads[0].Path)
}

func TestDownloadFutureDirLooksUpByPath(t *testing.T) {
	exec := &fakeExecutor{responses: map[string]string{
		"go mod": `{"Path":"example.com/foo","Version":"v1.0.0","Dir":"/cache/foo"}`,
	}}
	r := New(exec)

	f := r.startDownload(context.Background(), "/repo", nil)
	dir, err := f.dir(context.Background(), contentKey{path: "example.com/foo", version: "v1.0.0"})
	require.NoError(t, err)
	assert.Equal(t, "/cache/foo", dir)
}

func TestDownloadFutureDirReturnsErrorForUnknownPath(t *testing.T) {
	exec := &fakeExecutor{responses: map[string]string{
		"go mod": `{"Path":"example.com/foo","Version":"v1.0.0","Dir":"/cache/foo"}`,
	}}
	r := New(exec)

	f := r.startDownload(context.Background(), "/repo", nil)
	_, err := f.dir(context.Background(), contentKey{path: "example.com/missing", version: "v1.0.0"})
	assert.ErrorContains(t, err, "example.com/missing")
}

func TestDownloadFutureDirPropagatesDownloadError(t *testing.T) {
	// No "go mod" response configured: fakeExecutor.Run returns "unexpected
	// command", standing in for a real `go mod download` failure.
	exec := &fakeExecutor{}
	r := New(exec)

	f := r.startDownload(context.Background(), "/repo", nil)
	_, err := f.dir(context.Background(), contentKey{path: "example.com/foo", version: "v1.0.0"})
	assert.Error(t, err)
}

func TestDownloadFutureDirReturnsContextErrorWhenCancelledFirst(t *testing.T) {
	exec := blockingExecutor{args: make(chan []string, 1)}
	r := New(exec)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := r.startDownload(ctx, "/repo", nil)
	<-exec.args // wait until the download command has actually started
	cancel()

	_, err := f.dir(ctx, contentKey{path: "example.com/foo", version: "v1.0.0"})
	assert.ErrorIs(t, err, context.Canceled)
}

type vendorFailsDownloadBlocksExecutor struct {
	downloadCancelled chan struct{}
}

func (e *vendorFailsDownloadBlocksExecutor) Run(ctx context.Context, c Command) (string, error) {
	switch {
	case len(c.Args) >= 3 && c.Args[0] == "go" && c.Args[2] == "vendor":
		return "", errors.New("vendor failed")
	case len(c.Args) >= 3 && c.Args[0] == "go" && c.Args[1] == "mod" && c.Args[2] == "download":
		<-ctx.Done()
		close(e.downloadCancelled)
		return "", ctx.Err()
	default:
		return "", errors.New("unexpected command")
	}
}

func TestResolveModuleCancelsDownloadWhenVendorFailsFirst(t *testing.T) {
	exec := &vendorFailsDownloadBlocksExecutor{downloadCancelled: make(chan struct{})}
	dir := t.TempDir()
	goModPath := writeTestFile(t, dir, "go.mod", "module example.com/app\n\ngo 1.25.4\n")
	goMod, err := mod.ParseGoModFile(goModPath)
	require.NoError(t, err)

	r := New(exec)
	_, err = r.ResolveModule(context.Background(), goMod, nil)
	require.Error(t, err)

	select {
	case <-exec.downloadCancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("download's context was never cancelled after vendor failed — the goroutine leaked")
	}
}

// vendorStreamsThenFailsDownloadBlocksExecutor streams one full module —
// enough for vendorStreamHandler to submit a hash task for it — before
// vendor fails, standing in for a real toolchain that copies some modules
// successfully before hitting a fatal error partway through. The download
// command blocks until its context is cancelled, standing in for a slow or
// stuck `go mod download`.
type vendorStreamsThenFailsDownloadBlocksExecutor struct {
	downloadCancelled chan struct{}
}

func (e *vendorStreamsThenFailsDownloadBlocksExecutor) Run(ctx context.Context, c Command) (string, error) {
	switch {
	case len(c.Args) >= 3 && c.Args[0] == "go" && c.Args[2] == "vendor":
		c.OnStderr("# example.com/foo v1.0.0")
		c.OnStderr("## explicit")
		c.OnStderr("example.com/foo")
		return "", errors.New("vendor failed")
	case len(c.Args) >= 3 && c.Args[0] == "go" && c.Args[1] == "mod" && c.Args[2] == "download":
		<-ctx.Done()
		close(e.downloadCancelled)
		return "", ctx.Err()
	default:
		return "", errors.New("unexpected command")
	}
}

func TestResolveModuleReturnsPromptlyWhenVendorFailsWhileAHashTaskWaitsOnDownload(t *testing.T) {
	exec := &vendorStreamsThenFailsDownloadBlocksExecutor{downloadCancelled: make(chan struct{})}
	dir := t.TempDir()
	goModPath := writeTestFile(t, dir, "go.mod", "module example.com/app\n\ngo 1.25.4\n\nrequire example.com/foo v1.0.0\n")
	goMod, err := mod.ParseGoModFile(goModPath)
	require.NoError(t, err)

	r := New(exec)

	done := make(chan error, 1)
	go func() {
		_, err := r.ResolveModule(context.Background(), goMod, nil)
		done <- err
	}()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("ResolveModule did not return: a hash task blocked waiting on the download for example.com/foo's cache directory prevented the vendor failure from ever propagating")
	}

	select {
	case <-exec.downloadCancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("download's context was never cancelled")
	}
}

// orderTrackingExecutor calls onDownload/onVendor synchronously, before
// delegating to fakeExecutor, letting a test observe the relative order in
// which the two commands actually started running.
type orderTrackingExecutor struct {
	fakeExecutor
	onDownload func()
	onVendor   func()
}

func (e *orderTrackingExecutor) Run(ctx context.Context, c Command) (string, error) {
	switch {
	case len(c.Args) >= 3 && c.Args[0] == "go" && c.Args[1] == "mod" && c.Args[2] == "download":
		if e.onDownload != nil {
			e.onDownload()
		}
	case len(c.Args) >= 3 && c.Args[0] == "go" && c.Args[2] == "vendor":
		if e.onVendor != nil {
			e.onVendor()
		}
	}
	return e.fakeExecutor.Run(ctx, c)
}

func TestResolveModuleRunsDownloadConcurrentlyWithVendor(t *testing.T) {
	downloadStarted := make(chan struct{})
	releaseVendor := make(chan struct{})

	exec := &orderTrackingExecutor{
		fakeExecutor: fakeExecutor{
			responses: map[string]string{
				"go mod vendor": `# example.com/foo v1.0.0
## explicit
example.com/foo`,
				"go mod": `{"Path":"example.com/foo","Version":"v1.0.0","Dir":"testdata/module"}`,
			},
		},
		onDownload: func() { close(downloadStarted) },
		onVendor: func() {
			select {
			case <-downloadStarted:
			case <-time.After(time.Second):
				t.Error("go mod vendor ran to completion without go mod download ever starting — they are not running concurrently")
			}
			close(releaseVendor)
		},
	}

	dir := t.TempDir()
	goModPath := writeTestFile(t, dir, "go.mod", "module example.com/app\n\ngo 1.25.4\n\nrequire example.com/foo v1.0.0\n")
	goMod, err := mod.ParseGoModFile(goModPath)
	require.NoError(t, err)

	r := New(exec, WithHasher(&countingHasher{hash: "sha256-test"}))
	_, err = r.ResolveModule(context.Background(), goMod, nil)
	require.NoError(t, err)

	select {
	case <-releaseVendor:
	default:
		t.Fatal("vendor's onVendor hook never ran")
	}
}

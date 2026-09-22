package resolve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ModuleDownload is one entry from `go mod download -json`.
//
//nolint:tagliatelle
type ModuleDownload struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
	Dir     string `json:"Dir"`
	GoMod   string `json:"GoMod"`
	Error   string `json:"Error"`
}

// ParseDownloadOutput parses the JSON stream output of `go mod download -json`.
// Each JSON object is a separate module download result.
func ParseDownloadOutput(out string) ([]ModuleDownload, error) {
	var downloads []ModuleDownload
	dec := json.NewDecoder(strings.NewReader(out))
	for {
		var meta ModuleDownload
		if err := dec.Decode(&meta); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if meta.Error != "" {
			return nil, fmt.Errorf("failed to download %s@%s: %s", meta.Path, meta.Version, meta.Error)
		}
		downloads = append(downloads, meta)
	}
	return downloads, nil
}

// downloadFuture runs `go mod download -json` in the background, so it can
// overlap with `go mod vendor -v` instead of running strictly after it — by
// the time headers start streaming from vendor, every module is typically
// already fetched anyway (the toolchain's own download phase happens before
// it prints anything), so the two invocations mostly contend for local disk
// and module-cache locking rather than the network.
//
// done is closed exactly once, after downloads/err/byPath are all set, so a
// receive on done synchronizes-with those writes and later reads need no
// further locking.
type downloadFuture struct {
	done      chan struct{}
	downloads []ModuleDownload
	byPath    map[string]ModuleDownload
	err       error
}

// startDownload launches `go mod download -json` in dir and returns
// immediately; the result is available once the returned future's done
// channel closes. env is passed through to Command exactly as
// downloadModules would use it directly.
func (r *Resolver) startDownload(ctx context.Context, dir string, env []string) *downloadFuture {
	f := &downloadFuture{done: make(chan struct{})}
	go func() {
		defer close(f.done)
		f.downloads, f.err = r.downloadModules(ctx, dir, env)
		if f.err != nil {
			return
		}
		f.byPath = make(map[string]ModuleDownload, len(f.downloads))
		for _, dl := range f.downloads {
			f.byPath[dl.Path] = dl
		}
	}()
	return f
}

// wait blocks until the download completes, or ctx is done first.
func (f *downloadFuture) wait(ctx context.Context) ([]ModuleDownload, error) {
	select {
	case <-f.done:
		return f.downloads, f.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// dir implements dirResolver: it blocks until the download completes, then
// looks up the cache directory for key's path. Matches by path alone, like
// downloadModules' existing callers — a module's build has exactly one
// resolved version, so the download output never has two entries to
// disambiguate between.
func (f *downloadFuture) dir(ctx context.Context, key contentKey) (string, error) {
	select {
	case <-f.done:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if f.err != nil {
		return "", f.err
	}
	dl, ok := f.byPath[key.path]
	if !ok {
		return "", fmt.Errorf("module %s not found in download output", key.path)
	}
	return dl.Dir, nil
}

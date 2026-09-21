package resolve

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/purpleclay/conker/pool"
	"github.com/purpleclay/go-overlay/internal/modulestxt"
	"github.com/purpleclay/go-overlay/internal/progress"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingReporter records every event reported to it, in order. Safe for
// concurrent use: hash tasks report from pool goroutines while the vendor
// stream itself may still be feeding lines from another.
type recordingReporter struct {
	mu     sync.Mutex
	events []progress.Event
}

func (r *recordingReporter) Report(e progress.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *recordingReporter) kinds() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	kinds := make([]string, len(r.events))
	for i, e := range r.events {
		kinds[i] = fmt.Sprintf("%T", e)
	}
	return kinds
}

func TestVendorStreamHandlerClassifiesDownloading(t *testing.T) {
	reporter := &recordingReporter{}
	h := newVendorStreamHandler("go.mod", reporter, nil)

	h.line("go: downloading github.com/fatih/color v1.18.0")
	h.close()

	require.Len(t, reporter.events, 1)
	ev, ok := reporter.events[0].(progress.Downloading)
	require.True(t, ok)
	assert.Equal(t, "github.com/fatih/color", ev.Path)
	assert.Equal(t, "v1.18.0", ev.Version)
}

func TestVendorStreamHandlerClassifiesOtherGoAndWarningLinesAsNotes(t *testing.T) {
	reporter := &recordingReporter{}
	h := newVendorStreamHandler("go.mod", reporter, nil)

	h.line("go: finding module for package github.com/fatih/color")
	h.line("warning: some diagnostic")
	h.close()

	require.Len(t, reporter.events, 2)
	assert.Equal(t, []string{"progress.Note", "progress.Note"}, reporter.kinds())
}

func TestVendorStreamHandlerParsesRealWorldModulesTxtShapes(t *testing.T) {
	tests := []struct {
		name          string
		lines         []string
		wantVendored  []modulestxt.Module
		wantPhaseOnce bool
	}{
		{
			name: "plain module with tool directive package",
			lines: []string{
				"# github.com/sqlc-dev/doubleclick v1.0.0",
				"github.com/sqlc-dev/doubleclick/ast",
				"github.com/sqlc-dev/doubleclick/lexer",
				"# github.com/sqlc-dev/sqlc v1.31.1",
				"github.com/sqlc-dev/sqlc/cmd/sqlc",
			},
			wantVendored: []modulestxt.Module{
				{Path: "github.com/sqlc-dev/doubleclick", Version: "v1.0.0", Packages: []string{"github.com/sqlc-dev/doubleclick/ast", "github.com/sqlc-dev/doubleclick/lexer"}},
				{Path: "github.com/sqlc-dev/sqlc", Version: "v1.31.1", Packages: []string{"github.com/sqlc-dev/sqlc/cmd/sqlc"}},
			},
		},
		{
			name: "remote replace with trailer",
			lines: []string{
				"# gopkg.in/ini.v1 v1.67.0 => github.com/go-ini/ini v1.67.0",
				"## explicit",
				"gopkg.in/ini.v1",
				"# gopkg.in/ini.v1 => github.com/go-ini/ini v1.67.0",
			},
			wantVendored: []modulestxt.Module{
				{
					Path: "gopkg.in/ini.v1", Version: "v1.67.0", Explicit: true,
					Packages: []string{"gopkg.in/ini.v1"},
					Replace:  &modulestxt.Replace{Path: "github.com/go-ini/ini", Version: "v1.67.0"},
				},
			},
		},
		{
			name: "workspace marker",
			lines: []string{
				"## workspace",
				"# github.com/fatih/color v1.18.0",
				"## explicit; go 1.17",
				"github.com/fatih/color",
			},
			wantVendored: []modulestxt.Module{
				{Path: "github.com/fatih/color", Version: "v1.18.0", Explicit: true, GoVersion: "1.17", Packages: []string{"github.com/fatih/color"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := &recordingReporter{}
			h := newVendorStreamHandler("go.mod", reporter, nil)

			for _, line := range tt.lines {
				h.line(line)
			}
			h.close()

			var got []modulestxt.Module
			phaseChanges := 0
			for _, e := range reporter.events {
				switch ev := e.(type) {
				case progress.Vendored:
					got = append(got, modulestxt.Module{
						Path: ev.Path, Version: ev.Version,
					})
				case progress.PhaseChanged:
					phaseChanges++
					assert.Equal(t, progress.Vendor, ev.Phase)
				}
			}

			require.Len(t, got, len(tt.wantVendored))
			for i, want := range tt.wantVendored {
				assert.Equal(t, want.Path, got[i].Path)
				assert.Equal(t, want.Version, got[i].Version)
			}
			assert.Equal(t, 1, phaseChanges, "phase should change exactly once, on the first header")
		})
	}
}

func TestVendorStreamHandlerTreatsUnprefixedContinuationLinesAsNotes(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{name: "go.mod parse error detail line", line: `go.mod:5: require github.com/fatih/color: version "v99.99.99" invalid: should be v0 or v1, not v99`},
		{name: "tab-indented suggested command", line: "\tgo get github.com/caarlos0/env/v11@latest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := &recordingReporter{}
			h := newVendorStreamHandler("go.mod", reporter, nil)

			h.line("# github.com/fatih/color v1.18.0")
			h.line(tt.line)
			h.close()

			// The noise line is reported as a Note immediately (the module
			// header it followed is still open in the stream); Close then
			// flushes that module, producing PhaseChanged followed by
			// Vendored. This proves the noise line was never appended to
			// the module's package list.
			require.Len(t, reporter.events, 3, "expected Note, then PhaseChanged, then Vendored")
			_, ok := reporter.events[0].(progress.Note)
			require.True(t, ok, "noise line should be reported as a Note")

			vendored, ok := reporter.events[2].(progress.Vendored)
			require.True(t, ok)
			assert.Equal(t, "github.com/fatih/color", vendored.Path)
		})
	}
}

// signalingReporter wraps recordingReporter, additionally closing a
// per-path channel the moment a HashStarted for that path is reported. That
// lets a test wait — deterministically, no time.Sleep — for a hash task to
// have actually begun running on the pool, rather than merely submitted.
type signalingReporter struct {
	*recordingReporter
	hashStarted map[string]chan struct{}
}

func newSignalingReporter(watchPaths ...string) *signalingReporter {
	r := &signalingReporter{
		recordingReporter: &recordingReporter{},
		hashStarted:       make(map[string]chan struct{}, len(watchPaths)),
	}
	for _, path := range watchPaths {
		r.hashStarted[path] = make(chan struct{})
	}
	return r
}

func (r *signalingReporter) Report(e progress.Event) {
	r.recordingReporter.Report(e)
	if hs, ok := e.(progress.HashStarted); ok {
		if ch, ok := r.hashStarted[hs.Path]; ok {
			close(ch)
		}
	}
}

// waitForHashStarted blocks until path's HashStarted event has actually been
// reported, or fails the test if it never arrives within a generous bound.
func (r *signalingReporter) waitForHashStarted(t *testing.T, path string) {
	t.Helper()
	select {
	case <-r.hashStarted[path]:
	case <-time.After(2 * time.Second):
		t.Fatalf("HashStarted for %s was never reported", path)
	}
}

func indexOfEvent(events []progress.Event, match func(progress.Event) bool) int {
	for i, e := range events {
		if match(e) {
			return i
		}
	}
	return -1
}

func TestHashingOverlapsWithVendoring(t *testing.T) {
	reporter := newSignalingReporter("example.com/foo")
	dirFor := fakeDirResolver(map[contentKey]string{
		{path: "example.com/foo", version: "v1.0.0"}: "/cache/foo",
		{path: "example.com/bar", version: "v2.0.0"}: "/cache/bar",
	})
	scheduler := newHashScheduler(context.Background(), pool.New(), &countingHasher{hash: "sha256-test"}, reporter, "go.mod", nil, dirFor)
	handler := newVendorStreamHandler("go.mod", reporter, scheduler)

	handler.line("# example.com/foo v1.0.0")
	handler.line("## explicit")
	handler.line("example.com/foo")

	// bar's header flushes foo's entry, reporting Vendored(foo) and
	// dispatching foo's hash task to the pool — asynchronously, so nothing
	// here guarantees it has actually started yet.
	handler.line("# example.com/bar v2.0.0")

	// Block until it has, before the stream is allowed to proceed to bar's
	// remaining lines. A regression back to hashing only after the whole
	// vendor command exits would never report this, and the bounded wait
	// above would time out and fail the test rather than hang forever.
	reporter.waitForHashStarted(t, "example.com/foo")

	handler.line("## explicit")
	handler.line("example.com/bar")
	handler.close()
	scheduler.wait()

	events := reporter.events
	hashStartedFoo := indexOfEvent(events, func(e progress.Event) bool {
		hs, ok := e.(progress.HashStarted)
		return ok && hs.Path == "example.com/foo"
	})
	vendoredBar := indexOfEvent(events, func(e progress.Event) bool {
		v, ok := e.(progress.Vendored)
		return ok && v.Path == "example.com/bar"
	})

	require.GreaterOrEqual(t, hashStartedFoo, 0, "HashStarted for example.com/foo was never reported")
	require.GreaterOrEqual(t, vendoredBar, 0, "Vendored for example.com/bar was never reported")
	assert.Less(t, hashStartedFoo, vendoredBar,
		"HashStarted for foo must be reported before Vendored for bar — hashing must overlap with vendoring, not trail behind it")
}

package resolve

import (
	"fmt"
	"testing"

	"github.com/purpleclay/go-overlay/internal/modulestxt"
	"github.com/purpleclay/go-overlay/internal/progress"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingReporter records every event reported to it, in order.
type recordingReporter struct {
	events []progress.Event
}

func (r *recordingReporter) Report(e progress.Event) {
	r.events = append(r.events, e)
}

func (r *recordingReporter) kinds() []string {
	kinds := make([]string, len(r.events))
	for i, e := range r.events {
		kinds[i] = fmt.Sprintf("%T", e)
	}
	return kinds
}

func TestVendorStreamHandlerClassifiesDownloading(t *testing.T) {
	reporter := &recordingReporter{}
	h := newVendorStreamHandler("go.mod", reporter)

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
	h := newVendorStreamHandler("go.mod", reporter)

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
			h := newVendorStreamHandler("go.mod", reporter)

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
			h := newVendorStreamHandler("go.mod", reporter)

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

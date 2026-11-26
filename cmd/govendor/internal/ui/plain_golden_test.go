package ui

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/purpleclay/go-overlay/internal/progress"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/golden"
)

// eventLogDir holds the event logs that resolve's stream tests record from
// real `go mod vendor -v` stderr. Replaying the same files here ties the
// plain output to what resolve actually emits.
var eventLogDir = filepath.Join("..", "..", "..", "..", "internal", "resolve", "testdata", "stream")

func TestPlainReporterGolden(t *testing.T) {
	tests := []struct {
		name   string
		events string
		golden string
		opts   []Option
	}{
		{name: "cold download", events: "cold-download", golden: "cold-download", opts: []Option{WithInterval(0)}},
		{name: "cold download rate limited", events: "cold-download", golden: "cold-download-ratelimited"},
		{name: "local replace relative", events: "local-replace-relative", golden: "local-replace-relative", opts: []Option{WithInterval(0)}},
		{name: "local replace absolute", events: "local-replace-absolute", golden: "local-replace-absolute", opts: []Option{WithInterval(0)}},
		{name: "remote replace with trailer", events: "remote-replace", golden: "remote-replace", opts: []Option{WithInterval(0)}},
		{name: "workspace", events: "workspace", golden: "workspace", opts: []Option{WithInterval(0)}},
		{name: "failure", events: "failure", golden: "failure", opts: []Option{WithInterval(0)}},
		{name: "hash failure", events: "hash-failure", golden: "hash-failure", opts: []Option{WithInterval(0)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := NewPlainReporter(&buf, tt.opts...)
			for _, e := range readEventLog(t, tt.events) {
				r.Report(e)
			}

			golden.Assert(t, buf.String(), "stream/"+tt.golden+".golden")
		})
	}
}

func readEventLog(t *testing.T, name string) []progress.Event {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(eventLogDir, name+".events.golden"))
	require.NoError(t, err)

	var events []progress.Event
	for line := range strings.Lines(string(raw)) {
		f := strings.Split(strings.TrimSuffix(line, "\n"), "\t")
		target := progress.Target(f[1])
		switch f[0] {
		case "Started":
			events = append(events, progress.Started{Target: target, Expected: atoi(t, f[2]), Cold: atoi(t, f[3])})
		case "Downloading":
			events = append(events, progress.Downloading{Target: target, Path: f[2], Version: f[3]})
		case "Vendored":
			events = append(events, progress.Vendored{Target: target, Path: f[2], Version: f[3], Pkgs: atoi(t, f[4])})
		case "HashStarted":
			events = append(events, progress.HashStarted{Target: target, Path: f[2], Version: f[3]})
		case "Hashed":
			events = append(events, progress.Hashed{Target: target, Path: f[2], Version: f[3], Reused: f[4] == "true"})
		case "Note":
			events = append(events, progress.Note{Target: target, Text: f[2]})
		case "Finished":
			var err error
			if f[2] == "true" {
				err = errors.New("failed")
			}
			events = append(events, progress.Finished{Target: target, Err: err})
		default:
			t.Fatalf("unknown event %q in %s", f[0], name)
		}
	}
	return events
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	require.NoError(t, err)
	return n
}

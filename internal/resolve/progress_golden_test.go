package resolve

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/purpleclay/conker/pool"
	"github.com/purpleclay/go-overlay/internal/gocmd"
	"github.com/purpleclay/go-overlay/internal/progress"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/golden"
)

func TestStreamEvents(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		target  progress.Target
		err     error
	}{
		{name: "cold download", fixture: "cold-download.stderr", target: "go.mod"},
		{name: "local replace relative", fixture: "local-replace-relative.stderr", target: "go.mod"},
		{name: "local replace absolute", fixture: "local-replace-absolute.stderr", target: "go.mod"},
		{name: "remote replace with trailer", fixture: "remote-replace.stderr", target: "go.mod"},
		{name: "workspace", fixture: "workspace.stderr", target: "go.work"},
		{
			name:    "failure",
			fixture: "failure.stderr",
			target:  "go.mod",
			// A real vendor failure's error is an *ExecError carrying the
			// full captured stderr — the same text already streamed line by
			// line as Note events above.
			err: &gocmd.ExecError{Err: errors.New("exit status 1"), Stderr: mustReadFile(t, "failure.stderr")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := &recordingReporter{}

			reporter.Report(progress.Started{Target: tt.target})
			h := newVendorStreamHandler(tt.target, reporter, nil)
			for _, line := range readLines(t, tt.fixture) {
				h.line(line)
			}
			h.close()
			reporter.Report(progress.Finished{Target: tt.target, Err: tt.err})

			golden.Assert(t, eventLog(reporter.events), goldenName(tt.fixture))
		})
	}
}

func TestStreamEventsForAHashFailure(t *testing.T) {
	reporter := &recordingReporter{}

	const target progress.Target = "go.mod"
	key := contentKey{path: "example.com/foo", version: "v1.0.0"}
	hasher := &countingHasher{err: errors.New("disk on fire")}
	s := newHashScheduler(context.Background(), pool.New(), hasher, reporter, target, nil, fakeDirResolver(map[contentKey]string{key: "/cache/foo"}))

	reporter.Report(progress.Started{Target: target})
	s.submit(key)
	s.wait()
	result, _ := s.result(key)
	reporter.Report(progress.Finished{Target: target, Err: result.err})

	golden.Assert(t, eventLog(reporter.events), "stream/hash-failure.events.golden")
}

// eventLog renders events one per line, tab-separated, in the format
// govendor's ui tests replay through the plain reporter. Hashed.Took and
// the text of Finished.Err are left out: the first is wall-clock time, and
// the reporter only checks whether the second is set.
func eventLog(events []progress.Event) string {
	var b strings.Builder
	for _, e := range events {
		var fields []string
		switch ev := e.(type) {
		case progress.Started:
			fields = []string{"Started", string(ev.Target), fmt.Sprint(ev.Expected), fmt.Sprint(ev.Cold)}
		case progress.Downloading:
			fields = []string{"Downloading", string(ev.Target), ev.Path, ev.Version}
		case progress.Vendored:
			fields = []string{"Vendored", string(ev.Target), ev.Path, ev.Version, fmt.Sprint(ev.Pkgs)}
		case progress.HashStarted:
			fields = []string{"HashStarted", string(ev.Target), ev.Path, ev.Version}
		case progress.Hashed:
			fields = []string{"Hashed", string(ev.Target), ev.Path, ev.Version, fmt.Sprint(ev.Reused)}
		case progress.Note:
			fields = []string{"Note", string(ev.Target), ev.Text}
		case progress.Finished:
			fields = []string{"Finished", string(ev.Target), fmt.Sprint(ev.Err != nil)}
		default:
			panic(fmt.Sprintf("eventLog: unhandled event %T", e))
		}
		b.WriteString(strings.Join(fields, "\t") + "\n")
	}
	return b.String()
}

func readLines(t *testing.T, fixture string) []string {
	t.Helper()
	return strings.Split(strings.TrimRight(mustReadFile(t, fixture), "\n"), "\n")
}

func mustReadFile(t *testing.T, fixture string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "stream", fixture))
	require.NoError(t, err)
	return string(raw)
}

func goldenName(fixture string) string {
	return "stream/" + strings.TrimSuffix(fixture, ".stderr") + ".events.golden"
}

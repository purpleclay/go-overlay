package resolve

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/purpleclay/go-overlay/internal/progress"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/golden"
)

func TestPlainProgressOutput(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		manifest progress.Manifest
		err      error
	}{
		{name: "cold download", fixture: "cold-download.stderr", manifest: "go.mod"},
		{name: "local replace relative", fixture: "local-replace-relative.stderr", manifest: "go.mod"},
		{name: "local replace absolute", fixture: "local-replace-absolute.stderr", manifest: "go.mod"},
		{name: "remote replace with trailer", fixture: "remote-replace.stderr", manifest: "go.mod"},
		{name: "workspace", fixture: "workspace.stderr", manifest: "go.work"},
		{
			name:     "failure",
			fixture:  "failure.stderr",
			manifest: "go.mod",
			// A real vendor failure's error is an *ExecError carrying the
			// full captured stderr — the same text already streamed line by
			// line as Note events above. Using that real shape here, rather
			// than a short synthetic error, is what exposed the duplicated,
			// unprefixed second dump of it that PlainReporter used to print.
			err: &ExecError{Err: errors.New("exit status 1"), Stderr: mustReadFile(t, "failure.stderr")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			reporter := progress.NewPlainReporter(&buf, progress.WithInterval(0))

			reporter.Report(progress.Started{Manifest: tt.manifest})
			h := newVendorStreamHandler(tt.manifest, reporter)
			for _, line := range readLines(t, tt.fixture) {
				h.line(line)
			}
			h.close()
			reporter.Report(progress.Finished{Manifest: tt.manifest, Err: tt.err})

			golden.Assert(t, buf.String(), goldenName(tt.fixture))
		})
	}
}

func TestPlainProgressRateLimitedOutput(t *testing.T) {
	var buf bytes.Buffer
	reporter := progress.NewPlainReporter(&buf)

	const m progress.Manifest = "go.mod"
	reporter.Report(progress.Started{Manifest: m})
	h := newVendorStreamHandler(m, reporter)
	for _, line := range readLines(t, "cold-download.stderr") {
		h.line(line)
	}
	h.close()
	reporter.Report(progress.Finished{Manifest: m})

	golden.Assert(t, buf.String(), "stream/cold-download-ratelimited.golden")
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
	return "stream/" + strings.TrimSuffix(fixture, ".stderr") + ".golden"
}

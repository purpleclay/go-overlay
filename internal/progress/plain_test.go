package progress

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reporter returns a PlainReporter with a controllable clock, plus a lines
// func returning everything written so far, one entry per line. Asserting
// on the whole line sequence rather than substring containment means a test
// fails if the reporter writes something it shouldn't, not just if it fails
// to write something it should.
func reporter(t *testing.T) (r *PlainReporter, advance func(time.Duration), lines func() []string) {
	t.Helper()

	var buf bytes.Buffer
	now := time.Now()
	r = NewPlainReporter(&buf, withClock(func() time.Time { return now }))

	return r,
		func(d time.Duration) { now = now.Add(d) },
		func() []string {
			out := buf.String()
			if out == "" {
				return nil
			}
			return strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		}
}

func TestPlainReporterPhaseEventsPrintImmediately(t *testing.T) {
	r, _, lines := reporter(t)

	r.Report(Started{Manifest: "go.mod"})
	r.Report(PhaseChanged{Manifest: "go.mod", Phase: Vendor})
	r.Report(Note{Manifest: "go.mod", Text: "go: some diagnostic"})
	r.Report(Finished{Manifest: "go.mod"})

	assert.Equal(t, []string{
		"go.mod: resolving dependency graph",
		"go.mod: vendoring",
		"go.mod: go: some diagnostic",
	}, lines(), "Finished with a nil Err and no pending tally writes nothing")
}

func TestPlainReporterStartedCounts(t *testing.T) {
	tests := []struct {
		name  string
		event Started
		want  string
	}{
		{"unknown counts", Started{Manifest: "go.mod"}, "go.mod: resolving dependency graph"},
		{"all cached", Started{Manifest: "go.mod", Expected: 44}, "go.mod: resolving dependency graph (44 modules, all cached)"},
		{"partly cold", Started{Manifest: "go.mod", Expected: 44, Cold: 12}, "go.mod: resolving dependency graph (44 modules, 12 not cached)"},
		{"singular", Started{Manifest: "go.mod", Expected: 1, Cold: 1}, "go.mod: resolving dependency graph (1 module, 1 not cached)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _, lines := reporter(t)
			r.Report(tt.event)
			assert.Equal(t, []string{tt.want}, lines())
		})
	}
}

func TestPlainReporterReportsFinishedError(t *testing.T) {
	r, _, lines := reporter(t)

	r.Report(Finished{Manifest: "go.mod", Err: errors.New("boom")})

	assert.Equal(t, []string{"go.mod: failed"}, lines())
}

func TestPlainReporterFlushesPendingTallyBeforeAFailure(t *testing.T) {
	r, _, lines := reporter(t)

	for range 3 {
		r.Report(Downloading{Manifest: "go.mod"})
	}
	r.Report(Finished{Manifest: "go.mod", Err: errors.New("boom")})

	assert.Equal(t, []string{
		"go.mod: downloading modules (1 so far)",
		"go.mod: downloading modules (3 so far)",
		"go.mod: failed",
	}, lines(), "a failure should still record how far the run got")
}

func TestPlainReporterDoesNotRepeatAMultilineErrorOnFailure(t *testing.T) {
	r, _, lines := reporter(t)

	r.Report(Note{Manifest: "go.mod", Text: "go: errors parsing go.mod:"})
	r.Report(Note{Manifest: "go.mod", Text: "go.mod:5: unknown directive: bogus"})
	r.Report(Finished{Manifest: "go.mod", Err: errors.New("go: errors parsing go.mod:\ngo.mod:5: unknown directive: bogus")})

	assert.Equal(t, []string{
		"go.mod: go: errors parsing go.mod:",
		"go.mod: go.mod:5: unknown directive: bogus",
		"go.mod: failed",
	}, lines())
}

func TestPlainReporterRateLimitsPerModuleSummaries(t *testing.T) {
	r, advance, lines := reporter(t)

	for range 5 {
		r.Report(Downloading{Manifest: "go.mod"})
	}
	require.Equal(t, []string{"go.mod: downloading modules (1 so far)"}, lines(),
		"5 events within one rate-limit window produce one line; the first always prints")

	advance(summaryInterval)
	r.Report(Downloading{Manifest: "go.mod"})

	assert.Equal(t, []string{
		"go.mod: downloading modules (1 so far)",
		"go.mod: downloading modules (6 so far)",
	}, lines(), "the next printed summary reflects all 6 downloads, not just the triggering one")
}

func TestPlainReporterRateLimitIsPerManifest(t *testing.T) {
	r, _, lines := reporter(t)

	r.Report(Downloading{Manifest: "go.mod"})
	// A different manifest's first event must not be suppressed by
	// go.mod's rate limit, even though no time has passed.
	r.Report(Downloading{Manifest: "go.work"})

	assert.Equal(t, []string{
		"go.mod: downloading modules (1 so far)",
		"go.work: downloading modules (1 so far)",
	}, lines())
}

func TestPlainReporterVendoredAndHashedSummaries(t *testing.T) {
	r, advance, lines := reporter(t)

	r.Report(Vendored{Manifest: "go.mod", Pkgs: 3})
	advance(summaryInterval)
	r.Report(Hashed{Manifest: "go.mod", Reused: true})
	advance(summaryInterval)
	// HashStarted carries no output of its own.
	r.Report(HashStarted{Manifest: "go.mod"})

	assert.Equal(t, []string{
		"go.mod: vendored 1 module, 3 packages",
		"go.mod: hashing 1 module (1 reused)",
	}, lines())
}

func TestPlainReporterFlushesFinalTallyOnFinish(t *testing.T) {
	r, _, lines := reporter(t)

	for range 44 {
		r.Report(Downloading{Manifest: "go.mod"})
	}
	require.Equal(t, []string{"go.mod: downloading modules (1 so far)"}, lines(),
		"sanity check: the rate limit should have suppressed every count after the first")

	r.Report(Finished{Manifest: "go.mod"})

	assert.Equal(t, []string{
		"go.mod: downloading modules (1 so far)",
		"go.mod: downloading modules (44 so far)",
	}, lines())
}

func TestPlainReporterFlushesASuppressedTallyBeforeTheNextOne(t *testing.T) {
	r, _, lines := reporter(t)

	for range 44 {
		r.Report(Downloading{Manifest: "go.mod"})
	}
	r.Report(Vendored{Manifest: "go.mod", Pkgs: 2})
	r.Report(Finished{Manifest: "go.mod"})

	assert.Equal(t, []string{
		"go.mod: downloading modules (1 so far)",
		"go.mod: downloading modules (44 so far)",
		"go.mod: vendored 1 module, 2 packages",
	}, lines())
}

func TestPlainReporterFlushesASuppressedTallyOnPhaseChange(t *testing.T) {
	r, _, lines := reporter(t)

	for range 44 {
		r.Report(Downloading{Manifest: "go.mod"})
	}
	r.Report(PhaseChanged{Manifest: "go.mod", Phase: Hash})

	assert.Equal(t, []string{
		"go.mod: downloading modules (1 so far)",
		"go.mod: downloading modules (44 so far)",
		"go.mod: hashing",
	}, lines())
}

func TestPlainReporterFinishedDoesNotDuplicateAnAlreadyPrintedTally(t *testing.T) {
	r, _, lines := reporter(t)

	r.Report(Vendored{Manifest: "go.mod", Pkgs: 3})
	r.Report(Finished{Manifest: "go.mod"})

	assert.Equal(t, []string{"go.mod: vendored 1 module, 3 packages"}, lines())
}

func TestPlainReporterReleasesStateOnFinish(t *testing.T) {
	r, _, _ := reporter(t)

	r.Report(Downloading{Manifest: "go.mod"})
	r.Report(Finished{Manifest: "go.mod"})

	assert.Empty(t, r.state, "per-manifest state must not outlive its manifest under --recursive")
}

var lineShape = regexp.MustCompile(`^mod[0-3]/go\.mod: [a-z].*$`)

func TestPlainReporterConcurrentReportsAreSafe(t *testing.T) {
	r, _, lines := reporter(t)

	var wg sync.WaitGroup
	for m := range 4 {
		manifest := Manifest(fmt.Sprintf("mod%d/go.mod", m))
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				r.Report(Downloading{Manifest: manifest})
				r.Report(Vendored{Manifest: manifest, Pkgs: 1})
				r.Report(Hashed{Manifest: manifest})
			}
			r.Report(Finished{Manifest: manifest})
		}()
	}
	wg.Wait()

	out := lines()
	require.NotEmpty(t, out)
	for _, line := range out {
		assert.Regexp(t, lineShape, line, "a line was torn or interleaved")
	}
	assert.Empty(t, r.state)
}

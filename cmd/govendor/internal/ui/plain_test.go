package ui

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/purpleclay/go-overlay/internal/progress"
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

func TestPlainReporterMilestoneEventsPrintImmediately(t *testing.T) {
	r, _, lines := reporter(t)

	r.Report(progress.Started{Target: "go.mod"})
	r.Report(progress.Note{Target: "go.mod", Text: "go: some diagnostic"})
	r.Report(progress.Finished{Target: "go.mod"})

	assert.Equal(t, []string{
		"go.mod: resolving dependency graph",
		"go.mod: go: some diagnostic",
	}, lines(), "Finished with a nil Err and no pending tally writes nothing")
}

func TestPlainReporterStartedCounts(t *testing.T) {
	tests := []struct {
		name  string
		event progress.Started
		want  string
	}{
		{"unknown counts", progress.Started{Target: "go.mod"}, "go.mod: resolving dependency graph"},
		{"all cached", progress.Started{Target: "go.mod", Expected: 44}, "go.mod: resolving dependency graph (44 modules, all cached)"},
		{"partly cold", progress.Started{Target: "go.mod", Expected: 44, Cold: 12}, "go.mod: resolving dependency graph (44 modules, 12 not cached)"},
		{"singular", progress.Started{Target: "go.mod", Expected: 1, Cold: 1}, "go.mod: resolving dependency graph (1 module, 1 not cached)"},
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

	r.Report(progress.Finished{Target: "go.mod", Err: errors.New("boom")})

	assert.Equal(t, []string{"go.mod: failed"}, lines())
}

func TestPlainReporterFlushesPendingTallyBeforeAFailure(t *testing.T) {
	r, _, lines := reporter(t)

	for range 3 {
		r.Report(progress.Downloading{Target: "go.mod"})
	}
	r.Report(progress.Finished{Target: "go.mod", Err: errors.New("boom")})

	assert.Equal(t, []string{
		"go.mod: downloading modules (1 so far)",
		"go.mod: downloading modules (3 so far)",
		"go.mod: failed",
	}, lines(), "a failure should still record how far the run got")
}

func TestPlainReporterDoesNotRepeatAMultilineErrorOnFailure(t *testing.T) {
	r, _, lines := reporter(t)

	r.Report(progress.Note{Target: "go.mod", Text: "go: errors parsing go.mod:"})
	r.Report(progress.Note{Target: "go.mod", Text: "go.mod:5: unknown directive: bogus"})
	r.Report(progress.Finished{Target: "go.mod", Err: errors.New("go: errors parsing go.mod:\ngo.mod:5: unknown directive: bogus")})

	assert.Equal(t, []string{
		"go.mod: go: errors parsing go.mod:",
		"go.mod: go.mod:5: unknown directive: bogus",
		"go.mod: failed",
	}, lines())
}

func TestPlainReporterRateLimitsPerModuleSummaries(t *testing.T) {
	r, advance, lines := reporter(t)

	for range 5 {
		r.Report(progress.Downloading{Target: "go.mod"})
	}
	require.Equal(t, []string{"go.mod: downloading modules (1 so far)"}, lines(),
		"5 events within one rate-limit window produce one line; the first always prints")

	advance(summaryInterval)
	r.Report(progress.Downloading{Target: "go.mod"})

	assert.Equal(t, []string{
		"go.mod: downloading modules (1 so far)",
		"go.mod: downloading modules (6 so far)",
	}, lines(), "the next printed summary reflects all 6 downloads, not just the triggering one")
}

func TestPlainReporterRateLimitIsPerManifest(t *testing.T) {
	r, _, lines := reporter(t)

	r.Report(progress.Downloading{Target: "go.mod"})
	// A different manifest's first event must not be suppressed by
	// go.mod's rate limit, even though no time has passed.
	r.Report(progress.Downloading{Target: "go.work"})

	assert.Equal(t, []string{
		"go.mod: downloading modules (1 so far)",
		"go.work: downloading modules (1 so far)",
	}, lines())
}

func TestPlainReporterVendoredAndHashedSummaries(t *testing.T) {
	r, advance, lines := reporter(t)

	r.Report(progress.Vendored{Target: "go.mod", Pkgs: 3})
	advance(summaryInterval)
	r.Report(progress.Hashed{Target: "go.mod", Reused: true})
	advance(summaryInterval)
	// HashStarted carries no output of its own.
	r.Report(progress.HashStarted{Target: "go.mod"})

	assert.Equal(t, []string{
		"go.mod: vendoring",
		"go.mod: vendored 1 module, 3 packages",
		"go.mod: hashing 1 module (1 reused)",
	}, lines())
}

func TestPlainReporterFlushesFinalTallyOnFinish(t *testing.T) {
	r, _, lines := reporter(t)

	for range 44 {
		r.Report(progress.Downloading{Target: "go.mod"})
	}
	require.Equal(t, []string{"go.mod: downloading modules (1 so far)"}, lines(),
		"sanity check: the rate limit should have suppressed every count after the first")

	r.Report(progress.Finished{Target: "go.mod"})

	assert.Equal(t, []string{
		"go.mod: downloading modules (1 so far)",
		"go.mod: downloading modules (44 so far)",
	}, lines())
}

func TestPlainReporterFlushesASuppressedTallyBeforeTheNextOne(t *testing.T) {
	r, _, lines := reporter(t)

	for range 44 {
		r.Report(progress.Downloading{Target: "go.mod"})
	}
	r.Report(progress.Vendored{Target: "go.mod", Pkgs: 2})
	r.Report(progress.Finished{Target: "go.mod"})

	assert.Equal(t, []string{
		"go.mod: downloading modules (1 so far)",
		"go.mod: downloading modules (44 so far)",
		"go.mod: vendoring",
		"go.mod: vendored 1 module, 2 packages",
	}, lines())
}

func TestPlainReporterPrintsVendoringOnceOnTheFirstVendoredModule(t *testing.T) {
	r, _, lines := reporter(t)

	for range 44 {
		r.Report(progress.Downloading{Target: "go.mod"})
	}
	r.Report(progress.Vendored{Target: "go.mod", Pkgs: 1})
	r.Report(progress.Vendored{Target: "go.mod", Pkgs: 1})
	r.Report(progress.Finished{Target: "go.mod"})

	assert.Equal(t, []string{
		"go.mod: downloading modules (1 so far)",
		"go.mod: downloading modules (44 so far)",
		"go.mod: vendoring",
		"go.mod: vendored 2 modules, 2 packages",
	}, lines())
}

func TestPlainReporterRateLimitsInterleavedVendorAndHashTallies(t *testing.T) {
	r, advance, lines := reporter(t)

	for i := range 20 {
		r.Report(progress.Vendored{Target: "go.mod", Pkgs: 1})
		r.Report(progress.Hashed{Target: "go.mod", Reused: i%2 == 0})
	}

	require.Equal(t, []string{"go.mod: vendoring", "go.mod: vendored 1 module, 1 package"}, lines(),
		"vendoring and hashing interleaving within one window must not flush on every switch")

	advance(summaryInterval)
	r.Report(progress.Vendored{Target: "go.mod", Pkgs: 1})

	assert.Equal(t, []string{
		"go.mod: vendoring",
		"go.mod: vendored 1 module, 1 package",
		"go.mod: vendored 21 modules, 21 packages",
		"go.mod: hashing 20 modules (10 reused)",
	}, lines(), "once the window elapses, every tally with something pending flushes together, in a fixed order")
}

func TestPlainReporterFinishedDoesNotDuplicateAnAlreadyPrintedTally(t *testing.T) {
	r, _, lines := reporter(t)

	r.Report(progress.Vendored{Target: "go.mod", Pkgs: 3})
	r.Report(progress.Finished{Target: "go.mod"})

	assert.Equal(t, []string{"go.mod: vendoring", "go.mod: vendored 1 module, 3 packages"}, lines())
}

func TestPlainReporterReleasesStateOnFinish(t *testing.T) {
	r, _, _ := reporter(t)

	r.Report(progress.Downloading{Target: "go.mod"})
	r.Report(progress.Finished{Target: "go.mod"})

	assert.Empty(t, r.state, "per-manifest state must not outlive its manifest under --recursive")
}

var lineShape = regexp.MustCompile(`^mod[0-3]/go\.mod: [a-z].*$`)

func TestPlainReporterConcurrentReportsAreSafe(t *testing.T) {
	r, _, lines := reporter(t)

	var wg sync.WaitGroup
	for m := range 4 {
		manifest := progress.Target(fmt.Sprintf("mod%d/go.mod", m))
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				r.Report(progress.Downloading{Target: manifest})
				r.Report(progress.Vendored{Target: manifest, Pkgs: 1})
				r.Report(progress.Hashed{Target: manifest})
			}
			r.Report(progress.Finished{Target: manifest})
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

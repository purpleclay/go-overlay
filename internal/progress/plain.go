package progress

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// summaryInterval is the default minimum gap between per-module progress
// summaries (Downloading, Vendored, HashStarted, Hashed) for a single
// manifest. Phase-level events (Started, PhaseChanged, Note, Finished) are
// never rate-limited — they're rare enough that suppressing them would only
// lose information, not reduce noise.
const summaryInterval = 500 * time.Millisecond

// tally identifies which running count a summary belongs to.
type tally int

const (
	downloads tally = iota
	vendoring
	hashing
)

// tallyOrder is the fixed order flush prints pending tallies in, so a flush
// covering more than one is deterministic regardless of which one triggered
// it or what order their events actually arrived in.
var tallyOrder = [...]tally{downloads, vendoring, hashing}

// PlainReporter writes rate-limited, human-readable progress summaries to
// an io.Writer (stderr in normal use). It exists for CI logs and non-TTY
// output, where a per-module progress bar doesn't make sense but a large
// cold run looking like it's hung for minutes is still worth avoiding.
//
// Write errors are ignored: the writer is stderr, and failing a vendor run
// because a progress line couldn't be printed would be worse than silence.
// Must be created by NewPlainReporter.
type PlainReporter struct {
	w        io.Writer
	now      func() time.Time
	interval time.Duration

	// mu guards state and serialises writes to w, so concurrent manifests
	// can't interleave halves of a line.
	mu    sync.Mutex
	state map[Manifest]*manifestState
}

type manifestState struct {
	// pending holds each tally's most recently computed summary that hasn't
	// been written yet. Vendoring and hashing now happen concurrently per
	// module (issue 7's overlap), so two tallies can each have something
	// outstanding at once — a single shared slot would lose one of them.
	// lastSummary is when any tally for this manifest was last flushed,
	// shared across all of them, so an interleaved switch between tallies
	// doesn't itself reset the rate-limit window.
	pending     map[tally]string
	lastSummary time.Time

	downloaded int
	vendored   int
	packages   int
	hashed     int
	reused     int
}

// Option configures a PlainReporter.
type Option func(*PlainReporter)

// WithInterval sets the minimum gap between per-module progress summaries.
//
// Zero disables rate limiting: every summary is written as it is computed.
// That is what the golden tests use — with no rate limit there is no clock
// to fake and no wall-time dependence, so the captured output is a complete,
// deterministic record of the event stream rather than a timing-dependent
// sample of it.
func WithInterval(d time.Duration) Option {
	return func(r *PlainReporter) { r.interval = d }
}

// withClock overrides the reporter's clock. Unexported: only the rate
// limiter's own tests need it, and everything else should use WithInterval(0).
func withClock(now func() time.Time) Option {
	return func(r *PlainReporter) { r.now = now }
}

// NewPlainReporter returns a PlainReporter writing to w.
func NewPlainReporter(w io.Writer, opts ...Option) *PlainReporter {
	r := &PlainReporter{
		w:        w,
		now:      time.Now,
		interval: summaryInterval,
		state:    make(map[Manifest]*manifestState),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *PlainReporter) Report(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	m := e.Source()

	switch ev := e.(type) {
	case Started:
		r.print(m, startedText(ev))
	case PhaseChanged:
		// A phase boundary closes out any pending tally, so counts never
		// appear after the phase that produced them.
		r.flush(m)
		r.print(m, ev.Phase.String())
	case Downloading:
		st := r.stateFor(m)
		st.downloaded++
		r.summarize(m, st, downloads,
			fmt.Sprintf("downloading modules (%d so far)", st.downloaded))
	case Vendored:
		st := r.stateFor(m)
		st.vendored++
		st.packages += ev.Pkgs
		r.summarize(m, st, vendoring,
			fmt.Sprintf("vendored %s, %s", plural(st.vendored, "module"), plural(st.packages, "package")))
	case HashStarted:
		// No output on its own; Hashed carries the running tally.
	case Hashed:
		st := r.stateFor(m)
		st.hashed++
		if ev.Reused {
			st.reused++
		}
		r.summarize(m, st, hashing,
			fmt.Sprintf("hashing %s (%d reused)", plural(st.hashed, "module"), st.reused))
	case Note:
		r.print(m, ev.Text)
	case Finished:
		// Flush before reporting the outcome so the log ends on the true
		// final counts, including on failure, where how far it got matters.
		r.flush(m)
		if ev.Err != nil {
			// The error's own text is not printed here: for a streamed vendor
			// failure it's the whole captured stderr, which already reached
			// this log line by line as Note events. Repeating it would dump
			// the same diagnostics twice, the second time as one unprefixed
			// multi-line blob. Anyone needing the message itself still has
			// it via the results table or the command's own exit error.
			r.print(m, "failed")
		}
		delete(r.state, m)
	}
}

func (r *PlainReporter) stateFor(m Manifest) *manifestState {
	st, ok := r.state[m]
	if !ok {
		st = &manifestState{pending: make(map[tally]string, len(tallyOrder))}
		r.state[m] = st
	}
	return st
}

// summarize records text as m's latest summary for tally t. Once the
// rate-limit window has elapsed since the last flush for m — across every
// tally, not just t — everything currently pending is flushed together.
//
// The window is shared across tallies rather than reset whenever t changes:
// vendoring and hashing now interleave per module (issue 7's overlap), so
// treating every switch as its own trigger would flush on almost every
// event and defeat the rate limit entirely.
func (r *PlainReporter) summarize(m Manifest, st *manifestState, t tally, text string) {
	st.pending[t] = text

	if r.interval <= 0 {
		r.print(m, text)
		st.pending[t] = ""
		return
	}

	// A zero lastSummary saturates the subtraction to the maximum
	// Duration, so the very first summary for a manifest always writes.
	if now := r.now(); now.Sub(st.lastSummary) >= r.interval {
		st.lastSummary = now
		r.flush(m)
	}
}

// flush writes every tally's pending summary for m, in a fixed order, and
// clears them.
func (r *PlainReporter) flush(m Manifest) {
	st, ok := r.state[m]
	if !ok {
		return
	}
	for _, t := range tallyOrder {
		if text := st.pending[t]; text != "" {
			r.print(m, text)
			st.pending[t] = ""
		}
	}
}

func (r *PlainReporter) print(m Manifest, text string) {
	fmt.Fprintf(r.w, "%s: %s\n", m, text)
}

func startedText(ev Started) string {
	switch {
	case ev.Expected == 0:
		return "resolving dependency graph"
	case ev.Cold == 0:
		return fmt.Sprintf("resolving dependency graph (%s, all cached)", plural(ev.Expected, "module"))
	default:
		return fmt.Sprintf("resolving dependency graph (%s, %d not cached)", plural(ev.Expected, "module"), ev.Cold)
	}
}

// plural renders "1 module" / "3 modules", for units whose plural is a
// trailing s.
func plural(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

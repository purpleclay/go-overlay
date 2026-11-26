package progress

import "time"

// Target identifies which go.mod or go.work path an event belongs to, so a
// --recursive run resolving several targets concurrently can attribute each
// event to the right one.
//
// Every event type embeds Target, which is what makes them events: the
// Event interface is satisfied entirely by the methods below, so the
// individual event types carry no boilerplate of their own.
type Target string

// Source reports the target an event belongs to, letting a Reporter route
// events per target without a type switch.
func (t Target) Source() Target { return t }

func (Target) event() {}

// Event is implemented by every event type a Reporter can receive. The
// event method is unexported, so only types embedding Target can satisfy
// it and the set stays effectively closed to this package.
type Event interface {
	// Source reports the target this event belongs to.
	Source() Target
	event()
}

// Started marks the beginning of resolution for a target. Expected and
// Cold are best-effort counts of the target's remote modules and how many
// are missing from the module cache; both are zero until something (e.g.
// issue #12's pre-flight check) populates them.
type Started struct {
	Target
	Expected int
	Cold     int
}

// Downloading reports that the toolchain has started fetching a module that
// wasn't already in the module cache.
type Downloading struct {
	Target
	Path    string
	Version string
}

// Vendored reports that a module's packages have been copied into the
// throwaway vendor tree.
type Vendored struct {
	Target
	Path    string
	Version string
	Pkgs    int
}

// HashStarted reports that NAR hashing has begun for a module.
type HashStarted struct {
	Target
	Path    string
	Version string
}

// Hashed reports that a module's hash is available, either freshly computed
// or reused from an existing manifest/cache. It is only reported on success;
// a failed hash surfaces through Finished.Err instead.
type Hashed struct {
	Target
	Path    string
	Version string
	Reused  bool
	Took    time.Duration
}

// Note carries a toolchain diagnostic line (e.g. a "go: ..." message other
// than a download) that doesn't map to a more specific event.
type Note struct {
	Target
	Text string
}

// Finished marks the end of resolution for a target, successful or not.
// Exactly one Finished is emitted per target, and no further events for
// that target may follow it — Reporters may release per-target state on
// receipt.
type Finished struct {
	Target
	Err error
}

// Reporter receives progress events during resolution.
//
// Implementations must be safe for concurrent use: under --recursive,
// multiple targets are resolved concurrently and may report to the same
// Reporter at once. Report is called on the resolver's hot path, so
// implementations must not block on it for any appreciable time — an
// interactive view should hand the event to its own goroutine rather than
// render inline.
type Reporter interface {
	Report(Event)
}

var (
	_ Event    = Started{}
	_ Event    = Downloading{}
	_ Event    = Vendored{}
	_ Event    = HashStarted{}
	_ Event    = Hashed{}
	_ Event    = Note{}
	_ Event    = Finished{}
	_ Reporter = NopReporter{}
)

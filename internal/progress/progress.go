package progress

import (
	"fmt"
	"time"
)

// Manifest identifies which manifest (a go.mod or go.work path) an event
// belongs to, so a --recursive run resolving several manifests
// concurrently can attribute each event to the right one.
//
// Every event type embeds Manifest, which is what makes them events: the
// Event interface is satisfied entirely by the methods below, so the
// individual event types carry no boilerplate of their own.
type Manifest string

// Source reports the manifest an event belongs to, letting a Reporter route
// events per manifest without a type switch.
func (m Manifest) Source() Manifest { return m }

func (Manifest) event() {}

// Phase identifies which stage of resolution a manifest is currently in.
type Phase int

const (
	// Load is the implicit phase a manifest is in from Started until the
	// first modules.txt line arrives: process spawn, module download and
	// package-graph resolution, all silent on the toolchain's stderr. No
	// PhaseChanged is reported for it — Started marks its beginning.
	Load Phase = iota
	Vendor
	Hash
	Local
	// Write covers manifest writing, which happens in the vendor package
	// rather than the resolver. Nothing reports it yet: vendor.Vendor has
	// no Reporter plumbed through to it.
	Write
)

var phaseNames = [...]string{
	Load:   "loading",
	Vendor: "vendoring",
	Hash:   "hashing",
	Local:  "resolving local replacements",
	Write:  "writing manifest",
}

func (p Phase) String() string {
	if p < 0 || int(p) >= len(phaseNames) {
		return fmt.Sprintf("Phase(%d)", int(p))
	}
	return phaseNames[p]
}

// Event is implemented by every event type a Reporter can receive. The
// event method is unexported, so only types embedding Manifest can satisfy
// it and the set stays effectively closed to this package.
type Event interface {
	// Source reports the manifest this event belongs to.
	Source() Manifest
	event()
}

// Started marks the beginning of resolution for a manifest. Expected and
// Cold are best-effort counts of the manifest's remote modules and how many
// are missing from the module cache; both are zero until something (e.g.
// issue #12's pre-flight check) populates them.
type Started struct {
	Manifest
	Expected int
	Cold     int
}

// PhaseChanged marks a manifest moving into a new resolution phase.
type PhaseChanged struct {
	Manifest
	Phase Phase
}

// Downloading reports that the toolchain has started fetching a module that
// wasn't already in the module cache.
type Downloading struct {
	Manifest
	Path    string
	Version string
}

// Vendored reports that a module's packages have been copied into the
// throwaway vendor tree.
type Vendored struct {
	Manifest
	Path    string
	Version string
	Pkgs    int
}

// HashStarted reports that NAR hashing has begun for a module.
type HashStarted struct {
	Manifest
	Path    string
	Version string
}

// Hashed reports that a module's hash is available, either freshly computed
// or reused from an existing manifest/cache.
type Hashed struct {
	Manifest
	Path    string
	Version string
	Reused  bool
	Took    time.Duration
}

// Note carries a toolchain diagnostic line (e.g. a "go: ..." message other
// than a download) that doesn't map to a more specific event.
type Note struct {
	Manifest
	Text string
}

// Finished marks the end of resolution for a manifest, successful or not.
// Exactly one Finished is emitted per manifest, and no further events for
// that manifest may follow it — Reporters may release per-manifest state on
// receipt.
type Finished struct {
	Manifest
	Err error
}

// Reporter receives progress events during resolution.
//
// Implementations must be safe for concurrent use: under --recursive,
// multiple manifests are resolved concurrently and may report to the same
// Reporter at once. Report is called on the resolver's hot path, so
// implementations must not block on it for any appreciable time — an
// interactive view should hand the event to its own goroutine rather than
// render inline.
type Reporter interface {
	Report(Event)
}

var (
	_ Event    = Started{}
	_ Event    = PhaseChanged{}
	_ Event    = Downloading{}
	_ Event    = Vendored{}
	_ Event    = HashStarted{}
	_ Event    = Hashed{}
	_ Event    = Note{}
	_ Event    = Finished{}
	_ Reporter = NopReporter{}
	_ Reporter = (*PlainReporter)(nil)
)

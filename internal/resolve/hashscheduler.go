package resolve

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/purpleclay/conker/pool"
	"github.com/purpleclay/go-overlay/internal/mod"
	"github.com/purpleclay/go-overlay/internal/modulestxt"
	"github.com/purpleclay/go-overlay/internal/progress"
)

// contentKey identifies a downloaded module's content, independent of which
// declared path led to it: a remote replace's target and whatever else
// happens to require that same target are hashed once, not once per path.
type contentKey struct {
	path    string
	version string
}

// hashResult is the outcome of resolving one contentKey's hash, either
// reused from an existing manifest or computed fresh. goVersion carries the
// reused entry's own recorded go directive forward, if it has one — the
// same override resolveRemoteModule applied inline before this scheduler
// existed.
type hashResult struct {
	hash      string
	reused    bool
	goVersion string
	err       error
}

// isUnusedReplace reports whether m is a replace directive go.mod declares
// but nothing in the build actually requires: go mod vendor then records
// only a trailer-only summary for it — no packages, no annotation. Such an
// entry is never downloaded or hashed; its ModuleConfig comes entirely from
// the replace directive itself.
func isUnusedReplace(m modulestxt.Module) bool {
	return m.Replace != nil && m.Replace.Path != "" && len(m.Packages) == 0 && !m.Explicit && m.GoVersion == ""
}

// remoteContentKey returns the contentKey to hash m under, and whether m
// needs hashing at all. A local replacement is hashed from its working tree
// (see resolveLocalModule), not the module cache, so it never has one; an
// unused replace has nothing downloaded to hash either.
func remoteContentKey(m modulestxt.Module) (contentKey, bool) {
	if m.Replace != nil && m.Replace.Local != "" {
		return contentKey{}, false
	}
	if isUnusedReplace(m) {
		return contentKey{}, false
	}
	if m.Replace != nil && m.Replace.Path != "" {
		return contentKey{path: m.Replace.Path, version: m.Replace.Version}, true
	}
	return contentKey{path: m.Path, version: m.Version}, true
}

// dirResolver returns the module cache directory to hash for key, blocking
// until it's known — e.g. until `go mod download -json` has completed.
type dirResolver func(ctx context.Context, key contentKey) (string, error)

// hashScheduler dispatches at most one hash task per contentKey onto a
// shared pool, so a task can be submitted the moment a module's header
// streams from `go mod vendor -v` rather than after the command exits.
// Two different declared paths that resolve to the same content — e.g. a
// remote replace's target that something else also happens to require — are
// hashed once; both look up the same result.
//
// A scheduler is single-use: submit for the manifest it was built for, wait
// once, then read results. It never calls Wait on the pool itself, since the
// pool is shared with other schedulers and callers across a resolve.
type hashScheduler struct {
	// ctx is the caller's own context, captured at construction rather than
	// relied on from the pool. pool is shared across calls, so it carries
	// no single call's context of its own; each scheduler threads its caller's
	// ctx into the tasks it submits instead.
	//
	// It's derived with its own cancel so a failed vendor pass can abort
	// tasks still blocked in dirFor (a cache-miss download) rather than
	// waiting on them; see abort.
	ctx      context.Context
	cancel   context.CancelFunc
	pool     *pool.Pool
	hasher   Hasher
	reporter progress.Reporter
	manifest progress.Manifest
	dirFor   dirResolver

	// existing indexes the previous manifest by content identity — the
	// replacement target when there is one, otherwise the declared path —
	// rather than by declared path. The scheduler only ever has a
	// contentKey to check reuse against, not the original module that
	// requested it, so the index has to be keyed the same way.
	existing map[contentKey]mod.ModuleConfig

	mu        sync.Mutex
	submitted map[contentKey]struct{}
	results   map[contentKey]*hashResult
	wg        sync.WaitGroup
}

// newHashScheduler indexes existingMods by content identity and returns a
// scheduler ready to accept submissions. p is a shared pool owned by the
// caller; the scheduler submits tasks to it but never waits on it directly.
func newHashScheduler(ctx context.Context, p *pool.Pool, hasher Hasher, reporter progress.Reporter, manifest progress.Manifest, existingMods map[string]mod.ModuleConfig, dirFor dirResolver) *hashScheduler {
	existing := make(map[contentKey]mod.ModuleConfig, len(existingMods))
	for _, entry := range existingMods {
		if entry.Local != "" || entry.Hash == "" {
			continue
		}
		path := entry.Path
		if entry.ReplacedPath != "" {
			path = entry.ReplacedPath
		}
		existing[contentKey{path: path, version: entry.Version}] = entry
	}

	ctx, cancel := context.WithCancel(ctx)
	return &hashScheduler{
		ctx:       ctx,
		cancel:    cancel,
		pool:      p,
		hasher:    hasher,
		reporter:  reporter,
		manifest:  manifest,
		dirFor:    dirFor,
		existing:  existing,
		submitted: make(map[contentKey]struct{}),
		results:   make(map[contentKey]*hashResult),
	}
}

// submit dispatches resolution of key unless one was already submitted.
// Safe to call repeatedly with the same key, including concurrently from
// several modules that resolve to it.
//
// A reused hash needs neither dirFor nor the hasher, so it's settled
// synchronously right here — no goroutine, no pool slot. Anything else is
// handed to resolveAsync on a plain, unbounded goroutine rather than the
// pool: dirFor can block for an arbitrary amount of time (a cache-miss
// falls back to a targeted `go mod download`, see cachedModuleDir), and
// submit is called synchronously from the goroutine draining `go mod
// vendor -v`'s stderr. If that wait happened inside a pool slot, enough
// concurrently-waiting tasks would exhaust the pool and block the next
// submit — which blocks the stderr reader, which stalls `go mod vendor`
// itself on a full pipe. The pool is only ever asked for a slot once dirFor
// has already returned a real directory to hash.
func (s *hashScheduler) submit(key contentKey) {
	s.mu.Lock()
	if _, ok := s.submitted[key]; ok {
		s.mu.Unlock()
		return
	}
	s.submitted[key] = struct{}{}
	s.mu.Unlock()

	if entry, ok := s.existing[key]; ok {
		start := time.Now()
		s.reportStarted(key)
		result := hashResult{hash: entry.Hash, reused: true, goVersion: entry.GoVersion}
		s.reportHashed(key, result, time.Since(start))
		s.store(key, result)
		return
	}

	s.wg.Add(1)
	go s.resolveAsync(key)
}

// wait blocks until every task submitted so far has completed. Callers must
// not submit further keys for this scheduler afterwards.
func (s *hashScheduler) wait() {
	s.wg.Wait()
	s.cancel()
}

// abort cancels every in-flight task, so a following wait returns promptly
// instead of blocking on a cache-miss download whose result no longer
// matters. Safe to call more than once.
func (s *hashScheduler) abort() {
	s.cancel()
}

// result returns key's outcome. Only meaningful after wait has returned for
// a key that was actually submitted; the zero value and false otherwise.
func (s *hashScheduler) result(key contentKey) (hashResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.results[key]
	if !ok {
		return hashResult{}, false
	}
	return *r, true
}

// resolveAsync runs on its own goroutine, outside the pool, so waiting for
// dirFor never holds a slot. Only the hash computation itself — the part
// worth bounding — goes through the pool, via GoCtx so a cancelled context
// aborts the wait for a slot rather than queuing regardless.
//
// GoCtx dispatches the task to its own goroutine and returns as soon as a
// slot is acquired; it does not wait for the task to finish. hashed
// signals completion of the actual hash computation back to this goroutine,
// which is what actually needs to wait for it.
func (s *hashScheduler) resolveAsync(key contentKey) {
	defer s.wg.Done()

	start := time.Now()
	s.reportStarted(key)

	dir, err := s.dirFor(s.ctx, key)
	if err != nil {
		result := hashResult{err: err}
		s.reportHashed(key, result, time.Since(start))
		s.store(key, result)
		return
	}

	hashed := make(chan hashResult, 1)
	submitErr := s.pool.GoCtx(s.ctx, func(_ context.Context) error {
		// The pool recovers a panicking task on its own, but that recovery
		// is invisible here: without this recover, a panic in Hash would
		// leave nothing ever sent to hashed, so the receive below blocks
		// forever and this task's WaitGroup slot never closes.
		defer func() {
			if p := recover(); p != nil {
				hashed <- hashResult{err: fmt.Errorf("panic hashing module %s@%s: %v", key.path, key.version, p)}
			}
		}()
		hash, err := s.hasher.Hash(dir)
		if err != nil {
			hashed <- hashResult{err: fmt.Errorf("failed to hash downloaded module %s@%s: %w", key.path, key.version, err)}
			return nil
		}
		hashed <- hashResult{hash: hash}
		return nil
	})

	var result hashResult
	if submitErr != nil {
		// s.ctx was cancelled while waiting for a pool slot.
		result = hashResult{err: submitErr}
	} else {
		result = <-hashed
	}

	s.reportHashed(key, result, time.Since(start))
	s.store(key, result)
}

func (s *hashScheduler) reportStarted(key contentKey) {
	s.reporter.Report(progress.HashStarted{Manifest: s.manifest, Path: key.path, Version: key.version})
}

func (s *hashScheduler) reportHashed(key contentKey, result hashResult, took time.Duration) {
	s.reporter.Report(progress.Hashed{
		Manifest: s.manifest, Path: key.path, Version: key.version,
		Reused: result.reused, Took: took,
	})
}

func (s *hashScheduler) store(key contentKey, result hashResult) {
	s.mu.Lock()
	s.results[key] = &result
	s.mu.Unlock()
}

package resolve

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/purpleclay/conker/pool"
	"github.com/purpleclay/go-overlay/internal/mod"
	"github.com/purpleclay/go-overlay/internal/modulestxt"
	"github.com/purpleclay/go-overlay/internal/progress"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoteContentKeyExcludesLocalReplacements(t *testing.T) {
	m := modulestxt.Module{
		Path: "example.com/localmod", Version: "v0.0.0", Explicit: true,
		Replace: &modulestxt.Replace{Local: "./localmod"},
	}
	_, ok := remoteContentKey(m)
	assert.False(t, ok, "a local replacement is hashed from its working tree, not the module cache — it must never reach the hash scheduler")
}

func TestRemoteContentKeyExcludesUnusedReplacements(t *testing.T) {
	m := modulestxt.Module{
		Path:    "example.com/foo",
		Replace: &modulestxt.Replace{Path: "example.com/foo-fork", Version: "v9.9.9"},
	}
	_, ok := remoteContentKey(m)
	assert.False(t, ok, "an unused replace has nothing downloaded to hash")
}

func TestRemoteContentKeyUsesReplacementTargetForARemoteReplace(t *testing.T) {
	m := modulestxt.Module{
		Path: "example.com/foo", Version: "v1.0.0", Explicit: true,
		Packages: []string{"example.com/foo"},
		Replace:  &modulestxt.Replace{Path: "example.com/foo-fork", Version: "v2.0.0"},
	}
	key, ok := remoteContentKey(m)
	require.True(t, ok)
	assert.Equal(t, contentKey{path: "example.com/foo-fork", version: "v2.0.0"}, key)
}

func TestRemoteContentKeyUsesDeclaredPathWithNoReplace(t *testing.T) {
	m := modulestxt.Module{Path: "example.com/foo", Version: "v1.0.0", Explicit: true, Packages: []string{"example.com/foo"}}
	key, ok := remoteContentKey(m)
	require.True(t, ok)
	assert.Equal(t, contentKey{path: "example.com/foo", version: "v1.0.0"}, key)
}

func fakeDirResolver(dirs map[contentKey]string) dirResolver {
	return func(_ context.Context, key contentKey) (string, error) {
		dir, ok := dirs[key]
		if !ok {
			return "", errors.New("no dir configured for key")
		}
		return dir, nil
	}
}

func TestHashSchedulerComputesHashViaDirFor(t *testing.T) {
	hasher := &countingHasher{hash: "sha256-fresh"}
	key := contentKey{path: "example.com/foo", version: "v1.0.0"}
	dirFor := fakeDirResolver(map[contentKey]string{key: "/cache/foo"})

	s := newHashScheduler(context.Background(), pool.New(), hasher, progress.NopReporter{}, "go.mod", nil, dirFor)
	s.submit(key)
	s.wait()

	result, ok := s.result(key)
	require.True(t, ok)
	assert.NoError(t, result.err)
	assert.False(t, result.reused)
	assert.Equal(t, "sha256-fresh", result.hash)
	assert.Equal(t, int64(1), hasher.count.Load())
}

func TestHashSchedulerDedupesRepeatedSubmissions(t *testing.T) {
	hasher := &countingHasher{hash: "sha256-fresh"}
	key := contentKey{path: "example.com/foo", version: "v1.0.0"}
	dirFor := fakeDirResolver(map[contentKey]string{key: "/cache/foo"})

	s := newHashScheduler(context.Background(), pool.New(), hasher, progress.NopReporter{}, "go.mod", nil, dirFor)
	for range 5 {
		s.submit(key)
	}
	s.wait()

	assert.Equal(t, int64(1), hasher.count.Load(), "the same content key must only be hashed once")
}

func TestHashSchedulerHashesEachDistinctKeyOnce(t *testing.T) {
	hasher := &countingHasher{}
	foo := contentKey{path: "example.com/foo", version: "v1.0.0"}
	bar := contentKey{path: "example.com/bar", version: "v2.0.0"}
	dirFor := fakeDirResolver(map[contentKey]string{foo: "/cache/foo", bar: "/cache/bar"})

	s := newHashScheduler(context.Background(), pool.New(), hasher, progress.NopReporter{}, "go.mod", nil, dirFor)
	s.submit(foo)
	s.submit(bar)
	s.wait()

	assert.Equal(t, int64(2), hasher.count.Load())

	fooResult, ok := s.result(foo)
	require.True(t, ok)
	barResult, ok := s.result(bar)
	require.True(t, ok)
	assert.Equal(t, "sha256-/cache/foo", fooResult.hash)
	assert.Equal(t, "sha256-/cache/bar", barResult.hash)
}

func TestHashSchedulerReusesExistingManifestEntryByContentKey(t *testing.T) {
	hasher := &countingHasher{hash: "sha256-fresh"}
	key := contentKey{path: "example.com/foo", version: "v1.2.3"}
	existingMods := map[string]mod.ModuleConfig{
		"example.com/foo": {Path: "example.com/foo", Version: "v1.2.3", Hash: "sha256-cached"},
	}

	s := newHashScheduler(context.Background(), pool.New(), hasher, progress.NopReporter{}, "go.mod", existingMods, fakeDirResolver(nil))
	s.submit(key)
	s.wait()

	result, ok := s.result(key)
	require.True(t, ok)
	assert.True(t, result.reused)
	assert.Equal(t, "sha256-cached", result.hash)
	assert.Zero(t, hasher.count.Load(), "a reused entry must never call the hasher")
}

func TestHashSchedulerReuseAppliesAcrossDifferentAliasesOfTheSameContent(t *testing.T) {
	existingMods := map[string]mod.ModuleConfig{
		"example.com/old": {
			Path: "example.com/old", Version: "v1.0.0",
			ReplacedPath: "example.com/fork", Hash: "sha256-cached",
		},
	}
	hasher := &countingHasher{hash: "sha256-fresh"}
	forkKey := contentKey{path: "example.com/fork", version: "v1.0.0"}

	s := newHashScheduler(context.Background(), pool.New(), hasher, progress.NopReporter{}, "go.mod", existingMods, fakeDirResolver(nil))
	s.submit(forkKey)
	s.wait()

	result, ok := s.result(forkKey)
	require.True(t, ok)
	assert.True(t, result.reused, "the fork's content key must hit the entry recorded under example.com/old's replacement target, not example.com/old's own declared path")
	assert.Equal(t, "sha256-cached", result.hash)
}

func TestHashSchedulerIgnoresExistingEntryWithNoHashOrLocalOnly(t *testing.T) {
	hasher := &countingHasher{hash: "sha256-fresh"}
	key := contentKey{path: "example.com/foo", version: "v1.0.0"}
	dirFor := fakeDirResolver(map[contentKey]string{key: "/cache/foo"})

	existingMods := map[string]mod.ModuleConfig{
		"example.com/foo":   {Path: "example.com/foo", Version: "v1.0.0", Hash: ""},
		"example.com/local": {Path: "example.com/local", Version: "v1.0.0", Hash: "sha256-cached", Local: "./local"},
	}

	s := newHashScheduler(context.Background(), pool.New(), hasher, progress.NopReporter{}, "go.mod", existingMods, dirFor)
	s.submit(key)
	s.wait()

	result, ok := s.result(key)
	require.True(t, ok)
	assert.False(t, result.reused)
	assert.Equal(t, int64(1), hasher.count.Load())
}

func TestHashSchedulerPropagatesDirResolverError(t *testing.T) {
	hasher := &countingHasher{hash: "sha256-fresh"}
	key := contentKey{path: "example.com/foo", version: "v1.0.0"}

	s := newHashScheduler(context.Background(), pool.New(), hasher, progress.NopReporter{}, "go.mod", nil, fakeDirResolver(nil))
	s.submit(key)
	s.wait()

	result, ok := s.result(key)
	require.True(t, ok)
	assert.Error(t, result.err)
	assert.Zero(t, hasher.count.Load())
}

func TestHashSchedulerPropagatesHasherError(t *testing.T) {
	hasher := &countingHasher{err: errors.New("boom")}
	key := contentKey{path: "example.com/foo", version: "v1.0.0"}
	dirFor := fakeDirResolver(map[contentKey]string{key: "/cache/foo"})

	s := newHashScheduler(context.Background(), pool.New(), hasher, progress.NopReporter{}, "go.mod", nil, dirFor)
	s.submit(key)
	s.wait()

	result, ok := s.result(key)
	require.True(t, ok)
	assert.ErrorContains(t, result.err, "boom")
}

type panickingHasher struct{}

func (panickingHasher) Hash(string) (string, error) { panic("boom") }

func (panickingHasher) HashGitTracked(string, map[string]struct{}) (string, error) {
	panic("boom")
}

func TestHashSchedulerRecoversFromAPanickingHasher(t *testing.T) {
	key := contentKey{path: "example.com/foo", version: "v1.0.0"}
	dirFor := fakeDirResolver(map[contentKey]string{key: "/cache/foo"})

	s := newHashScheduler(context.Background(), pool.New(), panickingHasher{}, progress.NopReporter{}, "go.mod", nil, dirFor)
	s.submit(key)

	done := make(chan struct{})
	go func() {
		s.wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler.wait() hung after a panicking hasher — the WaitGroup slot was never released")
	}

	result, ok := s.result(key)
	require.True(t, ok)
	assert.Error(t, result.err)
}

func TestHashSchedulerReportsStartedThenHashedEvents(t *testing.T) {
	hasher := &countingHasher{hash: "sha256-fresh"}
	key := contentKey{path: "example.com/foo", version: "v1.0.0"}
	dirFor := fakeDirResolver(map[contentKey]string{key: "/cache/foo"})
	reporter := &recordingReporter{}

	s := newHashScheduler(context.Background(), pool.New(), hasher, reporter, "go.mod", nil, dirFor)
	s.submit(key)
	s.wait()

	require.Equal(t, []string{"progress.HashStarted", "progress.Hashed"}, reporter.kinds())
	started := reporter.events[0].(progress.HashStarted)
	assert.Equal(t, "example.com/foo", started.Path)
	hashed := reporter.events[1].(progress.Hashed)
	assert.Equal(t, "example.com/foo", hashed.Path)
	assert.False(t, hashed.Reused)
}

func TestHashSchedulerConcurrentSubmissionsAreSafe(t *testing.T) {
	hasher := &countingHasher{hash: "sha256-fresh"}
	key := contentKey{path: "example.com/mod", version: "v1.0.0"}
	dirFor := fakeDirResolver(map[contentKey]string{key: "/cache/mod"})

	s := newHashScheduler(context.Background(), pool.New(), hasher, progress.NopReporter{}, "go.mod", nil, dirFor)

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			s.submit(key)
		})
	}
	wg.Wait()
	s.wait()

	assert.Equal(t, int64(1), hasher.count.Load())
	result, ok := s.result(key)
	require.True(t, ok)
	assert.Equal(t, "sha256-fresh", result.hash)
}

type maxConcurrentHasher struct {
	mu      sync.Mutex
	current int
	max     int
}

func (h *maxConcurrentHasher) Hash(_ string) (string, error) {
	h.mu.Lock()
	h.current++
	if h.current > h.max {
		h.max = h.current
	}
	h.mu.Unlock()

	time.Sleep(20 * time.Millisecond)

	h.mu.Lock()
	h.current--
	h.mu.Unlock()
	return "sha256-x", nil
}

func (h *maxConcurrentHasher) HashGitTracked(dir string, _ map[string]struct{}) (string, error) {
	return h.Hash(dir)
}

func TestHashSchedulerBoundsConcurrentHashingByPoolSize(t *testing.T) {
	const poolSize = 3
	const keyCount = 12

	hasher := &maxConcurrentHasher{}
	keys := make([]contentKey, keyCount)
	dirs := make(map[contentKey]string, keyCount)
	for i := range keys {
		keys[i] = contentKey{path: fmt.Sprintf("example.com/mod%d", i), version: "v1.0.0"}
		dirs[keys[i]] = fmt.Sprintf("/cache/mod%d", i)
	}

	s := newHashScheduler(context.Background(), pool.New().WithMaxGoroutines(poolSize), hasher, progress.NopReporter{}, "go.mod", nil, fakeDirResolver(dirs))
	for _, key := range keys {
		s.submit(key)
	}
	s.wait()

	hasher.mu.Lock()
	defer hasher.mu.Unlock()
	assert.LessOrEqual(t, hasher.max, poolSize, "hashing concurrency must still be bounded by the pool size")
	assert.Equal(t, poolSize, hasher.max, "with far more keys than slots, the pool should actually reach saturation")
}

func blockingDirResolver(release <-chan struct{}, dirs map[contentKey]string) dirResolver {
	return func(ctx context.Context, key contentKey) (string, error) {
		select {
		case <-release:
		case <-ctx.Done():
			return "", ctx.Err()
		}
		dir, ok := dirs[key]
		if !ok {
			return "", errors.New("no dir configured for key")
		}
		return dir, nil
	}
}

func TestHashSchedulerSubmitNeverBlocksWaitingOnDirFor(t *testing.T) {
	const keyCount = 20
	release := make(chan struct{})
	keys := make([]contentKey, keyCount)
	dirs := make(map[contentKey]string, keyCount)
	for i := range keys {
		keys[i] = contentKey{path: fmt.Sprintf("example.com/mod%d", i), version: "v1.0.0"}
		dirs[keys[i]] = fmt.Sprintf("/cache/mod%d", i)
	}

	hasher := &countingHasher{hash: "sha256-test"}
	// A pool far smaller than keyCount: if submit ever held a slot while
	// waiting on dirFor, submitting past the pool's size would block once
	// every slot is occupied by a task stuck waiting on release.
	smallPool := pool.New().WithMaxGoroutines(2)
	s := newHashScheduler(context.Background(), smallPool, hasher, progress.NopReporter{}, "go.mod", nil, blockingDirResolver(release, dirs))

	submitted := make(chan struct{})
	go func() {
		for _, key := range keys {
			s.submit(key)
		}
		close(submitted)
	}()

	select {
	case <-submitted:
	case <-time.After(2 * time.Second):
		t.Fatal("submit blocked before every key could even be dispatched — a pool slot was held while waiting on dirFor")
	}

	close(release)
	s.wait()

	for _, key := range keys {
		result, ok := s.result(key)
		require.True(t, ok)
		assert.NoError(t, result.err)
		assert.Equal(t, "sha256-test", result.hash)
	}
}

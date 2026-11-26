package generate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"

	"github.com/purpleclay/conker/pool"
	"github.com/purpleclay/go-overlay/cmd/govendor/internal/discover"
	"github.com/purpleclay/go-overlay/internal/gomod"
	"github.com/purpleclay/go-overlay/internal/manifest"
)

type options struct {
	detectDrift bool
}

type Option func(*options)

func WithDriftDetection() Option {
	return func(opts *options) {
		opts.detectDrift = true
	}
}

// Resolver resolves Go module dependencies. Run delegates all toolchain
// interaction to a Resolver, which keeps this package free of
// process-execution concerns and lets it be exercised against fakes.
type Resolver interface {
	ResolveModule(ctx context.Context, goMod *gomod.File, existingMods map[string]manifest.Module) ([]manifest.Module, error)
	ResolveWorkspace(ctx context.Context, goWork *gomod.WorkFile, members []gomod.WorkspaceMember, existingMods map[string]manifest.Module) ([]manifest.Module, error)
}

// ErrFailed indicates one or more results in the rendered table failed.
// It carries no information beyond what the table already shows, so callers
// that have rendered the results table may choose to suppress its message.
var ErrFailed = errors.New("vendor failed")

// Run generates, or with drift detection checks, the govendor.toml of every
// target, concurrently. Results are sorted by path.
func Run(ctx context.Context, resolver Resolver, targets []discover.Target, opts ...Option) ([]Result, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	var results []Result
	p := pool.NewWithResults[Result]().WithContext(ctx)
	for _, t := range targets {
		if t.Err != nil {
			results = append(results, failedTarget(t))
			continue
		}
		p.Go(func(ctx context.Context) (Result, error) {
			return processTarget(ctx, resolver, t, o.detectDrift), nil
		})
	}

	processed, _ := p.Wait()
	results = append(results, processed...)

	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})

	for _, r := range results {
		if r.Status.IsFailure() {
			return results, ErrFailed
		}
	}

	return results, nil
}

func failedTarget(t discover.Target) Result {
	if errors.Is(t.Err, discover.ErrNoManifest) {
		return resultMissing(t.Path)
	}
	return resultError(t.Path, t.Err)
}

// processTarget implements the common drift detection and generation
// algorithm for modules and workspaces. It always runs full resolution and
// compares the resulting manifest against the existing one — byte-for-byte
// equality is the drift signal. This catches all classes of change including
// package list updates, not just go.mod-level directives.
func processTarget(ctx context.Context, resolver Resolver, t discover.Target, detectDrift bool) Result {
	var existingMods map[string]manifest.Module

	if t.Existing == nil {
		if detectDrift {
			return resultMissing(t.Path)
		}
	} else {
		existing, err := manifest.Parse(t.Existing)
		if err != nil {
			return resultError(t.Path, err)
		}
		if existing.Schema != manifest.SchemaVersion && detectDrift {
			return resultSchemaMismatch(t.Path, existing.Schema, manifest.SchemaVersion)
		}
		existingMods = existing.Mod
	}

	deps, rawTools, excludes, err := resolveTarget(ctx, resolver, t, existingMods)
	if err != nil {
		return resultError(t.Path, err)
	}

	var workspace *manifest.Workspace
	if t.Kind == discover.Workspace {
		workspace = t.Work.WorkspaceConfig()
	}

	newData, depCount, err := generate(deps, rawTools, excludes, workspace)
	if err != nil {
		return resultError(t.Path, err)
	}

	unchanged := t.Existing != nil && bytes.Equal(newData, t.Existing)

	if detectDrift {
		if unchanged {
			return resultOK(t.Path)
		}
		return resultDrift(t.Path)
	}

	if unchanged {
		return resultOK(t.Path)
	}

	if err := atomicWrite(filepath.Join(filepath.Dir(t.Path), manifest.Filename), newData); err != nil {
		return resultError(t.Path, err)
	}

	return resultGenerated(t.Path, depCount)
}

// resolveTarget dispatches to the resolver for the target's kind and returns
// the raw inputs needed to build a manifest.
func resolveTarget(ctx context.Context, resolver Resolver, t discover.Target, existingMods map[string]manifest.Module) (deps []manifest.Module, rawTools []string, excludes map[string][]string, err error) {
	switch t.Kind {
	case discover.Module:
		deps, err = resolver.ResolveModule(ctx, t.Mod, existingMods)
		rawTools = t.Mod.Tools
		if len(t.Mod.Excludes) > 0 {
			excludes = t.Mod.Excludes
		}
	case discover.Workspace:
		var members []gomod.WorkspaceMember
		members, err = t.Work.ParseMembers()
		if err != nil {
			return
		}
		deps, err = resolver.ResolveWorkspace(ctx, t.Work, members, existingMods)
		if err != nil {
			return
		}
		merged := make(map[string][]string)
		for _, m := range members {
			rawTools = append(rawTools, m.Tools...)
			for path, versions := range m.Excludes {
				merged[path] = append(merged[path], versions...)
			}
		}
		for path, versions := range merged {
			slices.Sort(versions)
			merged[path] = slices.Compact(versions)
		}
		if len(merged) > 0 {
			excludes = merged
		}
	default:
		err = fmt.Errorf("unsupported target kind: %d", t.Kind)
	}
	return
}

// generate builds and serialises a manifest from already-resolved dependency
// data. It has no knowledge of the target kind.
func generate(deps []manifest.Module, rawTools []string, excludes map[string][]string, workspace *manifest.Workspace) ([]byte, int, error) {
	// Build a package→version lookup from resolved deps so each tool entry
	// records its own module version rather than the application version.
	var tool manifest.Tools
	if len(rawTools) > 0 {
		pkgToVersion := make(map[string]string)
		for _, dep := range deps {
			for _, pkg := range dep.Packages {
				pkgToVersion[pkg] = dep.Version
			}
		}
		slices.Sort(rawTools)
		rawTools = slices.Compact(rawTools)
		tool = make(manifest.Tools, len(rawTools))
		for _, pkg := range rawTools {
			tool[pkg] = manifest.Tool{Version: pkgToVersion[pkg]}
		}
	}

	m := manifest.New(deps, workspace, tool, excludes)

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		return nil, 0, err
	}

	return buf.Bytes(), len(m.Mod), nil
}

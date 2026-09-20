package resolve

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/purpleclay/conker/pool"
	"github.com/purpleclay/go-overlay/internal/mod"
	"github.com/purpleclay/go-overlay/internal/modulestxt"
	"github.com/purpleclay/go-overlay/internal/progress"
	"golang.org/x/mod/semver"
)

// maxHashGoroutines bounds the hashing fan-out. NAR hashing is IO-bound on
// the module cache, so more goroutines than this stop helping.
const maxHashGoroutines = 8

// Resolver resolves Go module dependencies via the Go toolchain. All external
// commands go through the Executor interface, making the resolver testable
// with injected output.
type Resolver struct {
	exec     Executor
	hasher   Hasher
	reporter progress.Reporter
}

// Option configures a Resolver.
type Option func(*Resolver)

// WithReporter sets the progress.Reporter a Resolver reports resolution
// events to. Defaults to progress.NopReporter, so callers that don't care
// about progress reporting see no behaviour change.
func WithReporter(reporter progress.Reporter) Option {
	return func(r *Resolver) {
		if reporter != nil {
			r.reporter = reporter
		}
	}
}

// WithHasher sets the Hasher used for both downloaded and local modules.
func WithHasher(hasher Hasher) Option {
	return func(r *Resolver) {
		if hasher != nil {
			r.hasher = hasher
		}
	}
}

// New creates a Resolver with the given executor.
func New(exec Executor, opts ...Option) *Resolver {
	r := &Resolver{exec: exec, hasher: NARHasher{}, reporter: progress.NopReporter{}}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// ResolveModule resolves all dependencies for a single Go module.
// existingMods is the parsed existing manifest's module map; pass nil for a
// cold run.
//
// Started and Finished bracket the whole resolve, not just the vendor step:
// downloading and hashing account for most of the wall time on a cold run,
// and a Reporter that saw Finished before either began would stop reporting
// exactly when the user most needs to know the process isn't wedged.
func (r *Resolver) ResolveModule(ctx context.Context, goMod *mod.GoModFile, existingMods map[string]mod.ModuleConfig) ([]mod.ModuleConfig, error) {
	manifest := progress.Manifest(filepath.Join(goMod.Dir, mod.GoModFilename))
	r.reporter.Report(progress.Started{Manifest: manifest})

	modules, err := r.resolveModule(ctx, manifest, goMod, existingMods)
	r.reporter.Report(progress.Finished{Manifest: manifest, Err: err})
	return modules, err
}

func (r *Resolver) resolveModule(ctx context.Context, manifest progress.Manifest, goMod *mod.GoModFile, existingMods map[string]mod.ModuleConfig) ([]mod.ModuleConfig, error) {
	vendored, err := r.vendorModules(ctx, manifest, goMod.Dir, []string{"GOWORK=off"}, "mod")
	if err != nil {
		return nil, err
	}

	downloads, err := r.downloadModules(ctx, goMod.Dir, []string{"GOWORK=off"})
	if err != nil {
		return nil, err
	}

	pkgsByMod, remoteModules := splitVendored(vendored)

	modules, err := r.resolveRemoteModules(ctx, remoteModules, downloads, existingMods, goMod.Replacements)
	if err != nil {
		return nil, err
	}

	localModules, err := r.resolveLocalModules(ctx, localModulesOf(goMod, goMod.Dir, goMod.Requires), pkgsByMod)
	if err != nil {
		return nil, err
	}

	modules = append(modules, localModules...)
	sortModules(modules)
	return modules, nil
}

// ResolveWorkspace resolves dependencies across all modules in a Go workspace.
// A single go mod vendor pass from the workspace root replaces the per-platform
// go list fan-out. existingMods is the parsed existing manifest's module map;
// pass nil for a cold run.
func (r *Resolver) ResolveWorkspace(ctx context.Context, goWork *mod.GoWorkFile, existingMods map[string]mod.ModuleConfig) ([]mod.ModuleConfig, error) {
	manifest := progress.Manifest(filepath.Join(goWork.Dir, mod.GoWorkFilename))
	r.reporter.Report(progress.Started{Manifest: manifest})

	modules, err := r.resolveWorkspace(ctx, manifest, goWork, existingMods)
	r.reporter.Report(progress.Finished{Manifest: manifest, Err: err})
	return modules, err
}

func (r *Resolver) resolveWorkspace(ctx context.Context, manifest progress.Manifest, goWork *mod.GoWorkFile, existingMods map[string]mod.ModuleConfig) ([]mod.ModuleConfig, error) {
	members, err := goWork.ParseMembers()
	if err != nil {
		return nil, err
	}

	workspaceMembers := make(map[string]string, len(members))
	for _, m := range members {
		workspaceMembers[m.ModulePath] = m.Dir
	}

	// Single vendor pass from the workspace root with GOWORK active.
	vendored, err := r.vendorModules(ctx, manifest, goWork.Dir, nil, "work")
	if err != nil {
		return nil, err
	}

	// Download from the workspace root with GOWORK active so the Go toolchain
	// applies workspace-level MVS, producing one authoritative set of resolved
	// module versions rather than per-member independent resolutions.
	downloads, err := r.downloadModules(ctx, goWork.Dir, nil)
	if err != nil {
		return nil, err
	}

	pkgsByMod, remoteModules := splitVendored(vendored)

	// Parse each member go.mod once up front so both local replacement
	// passes can reuse the result without duplicate file I/O.
	memberGoMods := make(map[string]*mod.GoModFile, len(goWork.Modules))
	for _, modDir := range goWork.Modules {
		goModPath := filepath.Join(goWork.Dir, modDir, mod.GoModFilename)
		goMod, err := mod.ParseGoModFile(goModPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", goModPath, err)
		}
		memberGoMods[modDir] = goMod
	}

	// Merge member replace directives, then apply go.work's on top — go.work
	// replacements take precedence over member go.mod replacements (see
	// GoWorkFile.RemoteReplacements).
	replacements := make(map[string]mod.Replacement)
	for _, modDir := range goWork.Modules {
		for path, repl := range memberGoMods[modDir].Replacements {
			replacements[path] = repl
		}
	}
	for path, repl := range goWork.Replacements {
		replacements[path] = repl
	}

	remoteDeps, err := r.resolveRemoteModules(ctx, remoteModules, downloads, existingMods, replacements)
	if err != nil {
		return nil, err
	}

	allDeps := make(map[string]mod.ModuleConfig, len(remoteDeps))
	for _, dep := range remoteDeps {
		allDeps[dep.Path] = dep
	}

	// Resolve local replacements per member, preserving each member's base
	// directory for relative path resolution.
	var memberLocals []localModule
	for _, modDir := range goWork.Modules {
		goMod := memberGoMods[modDir]
		memberLocals = append(memberLocals, localModulesOf(goMod, goMod.Dir, goMod.Requires)...)
	}
	localDeps, err := r.resolveLocalModules(ctx, memberLocals, pkgsByMod)
	if err != nil {
		return nil, err
	}
	for _, dep := range localDeps {
		// Workspace-level remote replacements take precedence — do not
		// overwrite an entry already resolved from go.work replace directives.
		if _, exists := allDeps[dep.Path]; !exists {
			allDeps[dep.Path] = dep
		}
	}

	workspaceLocalDeps, err := r.resolveLocalModules(ctx,
		localModulesOf(goWork, goWork.Dir, workspaceVersions(goWork, memberGoMods, downloads)), pkgsByMod)
	if err != nil {
		return nil, err
	}
	for _, dep := range workspaceLocalDeps {
		allDeps[dep.Path] = dep
	}

	// Workspace members that are also required by other members appear in
	// downloads but not in modules.txt. Emit them as local source entries.
	downloadVersions := make(map[string]string, len(downloads))
	for _, dl := range downloads {
		downloadVersions[dl.Path] = dl.Version
	}
	for modulePath, localDir := range workspaceMembers {
		if existing, found := allDeps[modulePath]; found {
			// resolveLocalModule stores the path relative to the member's
			// directory (e.g. "../mood" from server/). Correct it to the
			// workspace-root-relative path so the builder can resolve it from
			// the govendor.toml location.
			existing.Local = localDir
			allDeps[modulePath] = existing
		} else if version, isDep := downloadVersions[modulePath]; isDep {
			allDeps[modulePath] = mod.ModuleConfig{
				Path:    modulePath,
				Version: version,
				Local:   localDir,
			}
		}
	}

	modules := make([]mod.ModuleConfig, 0, len(allDeps))
	for _, m := range allDeps {
		modules = append(modules, m)
	}
	sortModules(modules)
	return modules, nil
}

// localReplacer is the shared surface of GoModFile and GoWorkFile needed to
// enumerate local replace directives.
type localReplacer interface {
	LocalReplacements() []mod.Replacement
}

// localModulesOf pairs each local replace directive with the directory its
// relative path resolves against and the version to record for it.
func localModulesOf(src localReplacer, baseDir string, versions map[string]string) []localModule {
	repls := src.LocalReplacements()
	out := make([]localModule, 0, len(repls))
	for _, repl := range repls {
		out = append(out, localModule{repl: repl, baseDir: baseDir, version: versions[repl.OldPath]})
	}
	return out
}

// workspaceVersions picks the version to record for each workspace-level
// local replacement. The workspace build list wins (downloadModules from the
// root applies full MVS across all members and transitive dependencies);
// member requires — highest version — are the fallback for anything absent
// from it, such as a pure local-only replacement.
func workspaceVersions(goWork *mod.GoWorkFile, memberGoMods map[string]*mod.GoModFile, downloads []ModuleDownload) map[string]string {
	versions := make(map[string]string)
	for _, modDir := range goWork.Modules {
		for path, version := range memberGoMods[modDir].Requires {
			if existing, exists := versions[path]; !exists || semver.Compare(version, existing) > 0 {
				versions[path] = version
			}
		}
	}
	for _, dl := range downloads {
		versions[dl.Path] = dl.Version
	}
	return versions
}

// splitVendored indexes the vendored modules by path and separates out the
// remote ones, which are the only entries the download/hash pass handles.
func splitVendored(vendored []modulestxt.Module) (map[string][]string, []modulestxt.Module) {
	pkgsByMod := make(map[string][]string, len(vendored))
	remote := make([]modulestxt.Module, 0, len(vendored))
	for _, m := range vendored {
		pkgsByMod[m.Path] = m.Packages
		if m.Replace == nil || m.Replace.Local == "" {
			remote = append(remote, m)
		}
	}
	return pkgsByMod, remote
}

func sortModules(modules []mod.ModuleConfig) {
	sort.Slice(modules, func(i, j int) bool { return modules[i].Path < modules[j].Path })
}

// vendorModules runs go <verb> vendor -v -o <tmpdir> (verb is "mod" or
// "work"), parses the resulting modules.txt, and returns the ordered module
// list. The temp dir is always removed before this function returns.
//
// -v streams the modules.txt content to stderr as it's generated, which
// vendorStreamHandler classifies into progress events as they happen. The
// written modules.txt file remains the source of truth for the returned
// module list — the stream only drives progress reporting, so a stderr line
// the classifier can't confidently handle never risks corrupting the actual
// manifest data, only produces a stray Note event.
func (r *Resolver) vendorModules(ctx context.Context, manifest progress.Manifest, dir string, env []string, verb string) ([]modulestxt.Module, error) {
	tmpdir, err := os.MkdirTemp("", "govendor-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp vendor dir: %w", err)
	}
	defer os.RemoveAll(tmpdir)

	handler := newVendorStreamHandler(manifest, r.reporter)
	_, err = r.exec.Run(ctx, Command{
		Args:     []string{"go", verb, "vendor", "-v", "-o", tmpdir},
		Dir:      dir,
		Env:      env,
		OnStderr: handler.line,
	})
	handler.close()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(filepath.Join(tmpdir, "modules.txt"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open vendor modules.txt: %w", err)
	}
	defer f.Close()

	return modulestxt.Parse(f)
}

// downloadModules runs go mod download -json from dir. For a workspace, dir
// is the workspace root and env is nil so GOWORK stays active and the
// toolchain applies workspace-level MVS.
func (r *Resolver) downloadModules(ctx context.Context, dir string, env []string) ([]ModuleDownload, error) {
	out, err := r.exec.Run(ctx, Command{
		Args: []string{"go", "mod", "download", "-json"},
		Dir:  dir,
		Env:  env,
	})
	if err != nil {
		return nil, err
	}
	return ParseDownloadOutput(out)
}

// replacements is keyed by the original (left-hand) module path, sourced
// directly from go.mod's/go.work's replace directives — the only reliable
// way to know whether a directive was versioned (see mod.ModuleConfig.Versioned):
// modules.txt's header is byte-identical either way, so inferring this from
// modules.txt output alone is not possible.
func (r *Resolver) resolveRemoteModules(ctx context.Context, modules []modulestxt.Module, downloads []ModuleDownload, existingMods map[string]mod.ModuleConfig, replacements map[string]mod.Replacement) ([]mod.ModuleConfig, error) {
	if len(modules) == 0 {
		return nil, nil
	}

	dlByPath := make(map[string]ModuleDownload, len(downloads))
	for _, dl := range downloads {
		dlByPath[dl.Path] = dl
	}

	p := pool.NewWithResults[mod.ModuleConfig]().WithMaxGoroutines(maxHashGoroutines).WithContext(ctx)

	for _, m := range modules {
		p.Go(func(_ context.Context) (mod.ModuleConfig, error) {
			return r.resolveRemoteModule(m, dlByPath, existingMods, replacements)
		})
	}

	return p.Wait()
}

func (r *Resolver) resolveRemoteModule(m modulestxt.Module, dlByPath map[string]ModuleDownload, existingMods map[string]mod.ModuleConfig, replacements map[string]mod.Replacement) (mod.ModuleConfig, error) {
	path := m.Path

	// Unused replace: go.mod declares it, but nothing in the build
	// requires the original path, so go mod vendor recorded only a
	// trailer-only summary for it — no packages, no annotation,
	// whether or not modules.txt happens to show a version on the
	// left (that depends on whether the directive was versioned; see
	// mod.ModuleConfig.Versioned, sourced from go.mod directly below).
	// This can't be detected by whether the replacement path was
	// downloaded: it may have been anyway, coincidentally, because
	// something else independently requires it too — pkgsByMod's
	// entry for it stays populated by that unrelated module either way.
	if m.Replace != nil && m.Replace.Path != "" && len(m.Packages) == 0 && !m.Explicit && m.GoVersion == "" {
		repl := replacements[path]
		var requiredVersion string
		if repl.OldVersion != "" && repl.OldVersion != m.Replace.Version {
			requiredVersion = repl.OldVersion
		}
		return mod.ModuleConfig{
			Path:            path,
			Version:         m.Replace.Version,
			RequiredVersion: requiredVersion,
			Versioned:       repl.OldVersion != "",
			ReplacedPath:    m.Replace.Path,
			Unused:          true,
		}, nil
	}

	var replacedPath string
	var meta ModuleDownload
	var ok bool

	if m.Replace != nil && m.Replace.Path != "" {
		// Remote replacement: the download is keyed by the replacement path.
		replacedPath = m.Replace.Path
		meta, ok = dlByPath[m.Replace.Path]
	} else {
		meta, ok = dlByPath[path]
	}

	if !ok {
		return mod.ModuleConfig{}, fmt.Errorf("module %s not found in download output", path)
	}

	// The version modules.txt reported as required, before the replace
	// was applied. Only meaningful — and only kept — when it diverges
	// from the replacement's resolved version.
	var requiredVersion string
	if replacedPath != "" && m.Version != meta.Version {
		requiredVersion = m.Version
	}

	// Whether go.mod's replace directive named a version on the left of
	// "=>" — see mod.ModuleConfig.Versioned. Read directly from the
	// parsed replace directive rather than inferred from modules.txt.
	var versioned bool
	if replacedPath != "" {
		versioned = replacements[path].OldVersion != ""
	}

	cfg := mod.ModuleConfig{
		Path:            path,
		Version:         meta.Version,
		RequiredVersion: requiredVersion,
		Versioned:       versioned,
		Packages:        m.Packages,
		GoVersion:       m.GoVersion,
		ReplacedPath:    replacedPath,
		Implicit:        !m.Explicit,
	}

	// Warm path: remote (path, version) pairs are immutable under the
	// checksum DB — reuse hash and go version from the existing manifest
	// when the (path, version, replacedPath) triple matches.
	if entry, ok := existingMods[path]; ok &&
		entry.Version == meta.Version &&
		entry.Local == "" &&
		entry.Hash != "" &&
		entry.ReplacedPath == replacedPath {
		cfg.Hash = entry.Hash
		if entry.GoVersion != "" {
			cfg.GoVersion = entry.GoVersion
		}
		return cfg, nil
	}

	// Cold path: hash the module.
	hash, err := r.hasher.Hash(meta.Dir)
	if err != nil {
		return mod.ModuleConfig{}, fmt.Errorf("failed to hash downloaded module %s@%s: %w", meta.Path, meta.Version, err)
	}
	cfg.Hash = hash
	return cfg, nil
}

// resolveLocalModules hashes and builds ModuleConfig entries for a set of
// local replace directives. Callers supply the base directory and version
// for each, so go.mod-level and go.work-level replacements share one code
// path instead of two near-identical copies.
func (r *Resolver) resolveLocalModules(ctx context.Context, locals []localModule, pkgsByMod map[string][]string) ([]mod.ModuleConfig, error) {
	if len(locals) == 0 {
		return nil, nil
	}

	p := pool.NewWithResults[mod.ModuleConfig]().WithMaxGoroutines(maxHashGoroutines).WithContext(ctx)
	for _, lm := range locals {
		p.Go(func(ctx context.Context) (mod.ModuleConfig, error) {
			return r.resolveLocalModule(ctx, lm, pkgsByMod)
		})
	}
	return p.Wait()
}

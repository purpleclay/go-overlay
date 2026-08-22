package mod

// ModuleConfig represents a single Go module dependency, both as the result
// of dependency resolution and as a row in the generated manifest.
type ModuleConfig struct {
	Path string `toml:"-"`
	// Version is the version that was fetched and hashed: the replacement's
	// version for a remote replace, or the plain required version otherwise.
	Version string `toml:"version"`
	Hash    string `toml:"hash,omitempty"`
	// RequiredVersion is the version go.mod originally required, before a
	// remote replace directive was applied. Only set when it differs from
	// Version — the common case (no replace, or a same-version replace)
	// leaves it empty.
	RequiredVersion string `toml:"required,omitempty"`
	// Versioned marks a remote replace whose go.mod directive named a version
	// on the left of "=>" (replace A vOld => B vNew), as opposed to the more
	// common form that omits it and so applies to every required version
	// (replace A => B vNew). The builder must omit the "# path => replaced
	// version" modules.txt trailer when Versioned is true — go mod vendor
	// never writes one for a versioned replace — but always emits it
	// otherwise, even when Version and RequiredVersion happen to match.
	Versioned    bool     `toml:"versioned,omitempty"`
	GoVersion    string   `toml:"go,omitempty"`
	Packages     []string `toml:"packages,omitempty"`
	ReplacedPath string   `toml:"replaced,omitempty"`
	Local        string   `toml:"local,omitempty"`
	Implicit     bool     `toml:"implicit,omitempty"`
	// Unused marks a remote wildcard replace that nothing in the build
	// actually requires. go.mod still declares it, but go mod vendor records
	// only the trailer form ("# A => B vN") — no primary header, annotation,
	// or packages — since there's nothing to attribute packages to and
	// nothing that needs B fetched. Version holds B's version (for the
	// trailer's right-hand side); Hash, GoVersion, Packages, and Implicit
	// are all meaningless here and stay empty.
	Unused bool `toml:"unused,omitempty"`
}

// WorkspaceConfig holds Go workspace metadata recorded in the manifest. It is
// also used to reconstruct a GoWorkFile when go.work is not committed.
type WorkspaceConfig struct {
	Go        string   `toml:"go"`
	Toolchain string   `toml:"toolchain,omitempty"`
	Modules   []string `toml:"modules"`
}

// ToolEntry records the resolved version of a single Go tool directive.
type ToolEntry struct {
	Version string `toml:"version"`
}

// ToolConfig records Go tool directive packages in the manifest, keyed by
// package path. The Nix builder uses this to compile each tool for the host
// platform, labelling each derivation with the tool's own module version.
type ToolConfig map[string]ToolEntry

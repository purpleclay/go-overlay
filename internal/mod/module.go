package mod

// ModuleConfig represents a single Go module dependency, both as the result
// of dependency resolution and as a row in the generated manifest.
type ModuleConfig struct {
	Path         string   `toml:"-"`
	Version      string   `toml:"version"`
	Hash         string   `toml:"hash,omitempty"`
	GoVersion    string   `toml:"go,omitempty"`
	Packages     []string `toml:"packages,omitempty"`
	ReplacedPath string   `toml:"replaced,omitempty"`
	Local        string   `toml:"local,omitempty"`
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

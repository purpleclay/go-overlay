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

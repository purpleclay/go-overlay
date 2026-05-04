package mod

// defaultPlatforms is the default set of GOOS/GOARCH pairs used for
// cross-platform dependency resolution.
var defaultPlatforms = []string{
	"linux/amd64",
	"linux/arm64",
	"darwin/amd64",
	"darwin/arm64",
	"windows/amd64",
	"windows/arm64",
}

// DefaultPlatforms returns a copy of the default GOOS/GOARCH pairs.
func DefaultPlatforms() []string {
	return append([]string(nil), defaultPlatforms...)
}

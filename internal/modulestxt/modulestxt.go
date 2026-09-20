// Package modulestxt parses the vendor/modules.txt format produced by
// go mod vendor and go work vendor.
package modulestxt

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"golang.org/x/mod/modfile"
)

// Module is one entry from a vendor/modules.txt file.
type Module struct {
	Path      string
	Version   string
	Explicit  bool
	GoVersion string
	Packages  []string
	// nil when no replacement directive is present
	Replace *Replace
}

// Replace holds the target of a replace directive. Exactly one of Path or
// Local is non-empty: Path for remote replacements, Local for local ones.
type Replace struct {
	// replacement module path (remote replace only)
	Path string
	// replacement module version (remote replace only)
	Version string
	// relative filesystem path (local replace only)
	Local string
}

// Parse reads a vendor/modules.txt file and returns the ordered list of
// modules it describes. The workspace header (## workspace) is accepted and
// ignored. Parse is tolerant of modules that have no package lines.
func Parse(r io.Reader) ([]Module, error) {
	var modules []Module
	var stream Stream
	stream.Emit = func(m Module) { modules = append(modules, m) }

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if err := stream.Line(scanner.Text()); err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	stream.Flush()

	return modules, nil
}

// parseHeader parses the content after the leading "# " on a module header
// line. Accepted forms:
//
//	path version
//	path version => newpath newversion
//	path version => ./dir
//	path => ./dir
func parseHeader(s string) (Module, error) {
	left, right, hasReplace := strings.Cut(s, " => ")

	parts := strings.Fields(left)
	var m Module
	switch len(parts) {
	case 1:
		// "path" — only valid in the wildcard local-replace form "path => ./dir"
		if !hasReplace {
			return Module{}, fmt.Errorf("modulestxt: missing version in header: %q", s)
		}
		m.Path = parts[0]
	case 2:
		m.Path = parts[0]
		m.Version = parts[1]
	default:
		return Module{}, fmt.Errorf("modulestxt: malformed module header: %q", s)
	}

	if !hasReplace {
		return m, nil
	}

	repl, err := parseReplace(right, s)
	if err != nil {
		return Module{}, err
	}
	m.Replace = &repl
	return m, nil
}

// parseReplace classifies the right-hand side of a "=>" as either a local
// directory or a remote module.
//
// Directory detection uses modfile.IsDirectoryPath, the same predicate the
// Go toolchain applies when parsing a replace directive. Testing for a
// leading "." is not enough: `replace x => /opt/lib` and, on Windows,
// `replace x => C:\src\lib` are both valid and both produce a modules.txt
// header the "." test rejects outright, failing the whole resolve for a
// vendor tree the toolchain itself wrote happily.
func parseReplace(s, header string) (Replace, error) {
	parts := strings.Fields(s)
	switch {
	case len(parts) == 0:
		return Replace{}, fmt.Errorf("modulestxt: malformed replacement in header: %q", header)
	case modfile.IsDirectoryPath(parts[0]):
		// A directory replacement never carries a version, but tolerate one
		// rather than fail the run over a field the resolver doesn't read.
		return Replace{Local: parts[0]}, nil
	case len(parts) == 2:
		return Replace{Path: parts[0], Version: parts[1]}, nil
	default:
		return Replace{}, fmt.Errorf("modulestxt: malformed replacement in header: %q", header)
	}
}

// parseAnnotation parses the content after "## " and updates the module in
// place. Accepted forms: "explicit", "go X.Y", "explicit; go X.Y".
func parseAnnotation(s string, m *Module) {
	for part := range strings.SplitSeq(s, "; ") {
		switch {
		case part == "explicit":
			m.Explicit = true
		case strings.HasPrefix(part, "go "):
			m.GoVersion = strings.TrimPrefix(part, "go ")
		}
	}
}

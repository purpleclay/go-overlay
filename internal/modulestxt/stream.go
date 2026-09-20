package modulestxt

import (
	"fmt"
	"strings"
)

// Stream incrementally parses vendor/modules.txt-shaped lines, calling Emit
// each time a complete module entry is available. Parse drives one by
// looping over a file's lines; the resolver's live stderr stream from
// `go <verb> vendor -v` drives another directly, so the exact same tested
// line-handling logic backs both instead of two parsers that could drift
// apart.
//
// The zero value is ready to use. Emit may be nil, in which case entries are
// parsed and discarded.
type Stream struct {
	// Emit is called synchronously from within Line and Flush, once per
	// completed module entry, in file order. Set it before the first Line.
	Emit func(Module)

	current *Module
	seen    map[string]bool
}

// Line processes one line of modules.txt-shaped content. Line must not be
// called with anything that isn't part of modules.txt's own format — a
// caller reading a mixed stream (e.g. `go`'s stderr under -v, which also
// carries "go: downloading ..." lines) is responsible for filtering those
// out first.
func (s *Stream) Line(line string) error {
	switch {
	case line == "## workspace":
		// workspace header — accepted, produces no output

	case strings.HasPrefix(line, "# "):
		s.Flush()

		m, err := parseHeader(line[2:])
		if err != nil {
			return err
		}
		// go mod vendor appends version-less "# path => replacement" trailer
		// lines after all module entries to summarise replace directives. Skip
		// them — the real versioned header for the same path was already parsed.
		// Version-less entries with a NEW path are either a wildcard local
		// replace header, or a remote replace that go.mod declares but that
		// nothing in the build actually requires — see resolve.go, which
		// reads go.mod's replace directives directly to tell these apart from
		// an ordinary module and from each other, rather than inferring it
		// from modules.txt's shape here.
		if s.seen[m.Path] && m.Version == "" {
			return nil
		}
		s.current = &m

	case strings.HasPrefix(line, "## "):
		if s.current == nil {
			return fmt.Errorf("modulestxt: annotation before module header: %q", line)
		}
		parseAnnotation(line[3:], s.current)

	case line != "":
		if s.current != nil {
			s.current.Packages = append(s.current.Packages, line)
		}
	}

	return nil
}

// Flush emits the in-progress module entry, if any. It must be called once
// after the last Line, or the final module in the file/stream is silently
// dropped. Flush is idempotent, and Line calls it at each module boundary.
func (s *Stream) Flush() {
	if s.current == nil {
		return
	}
	if s.seen == nil {
		s.seen = make(map[string]bool)
	}
	s.seen[s.current.Path] = true
	if s.Emit != nil {
		s.Emit(*s.current)
	}
	s.current = nil
}

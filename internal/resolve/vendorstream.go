package resolve

import (
	"strings"

	"github.com/purpleclay/go-overlay/internal/modulestxt"
	"github.com/purpleclay/go-overlay/internal/progress"
)

// vendorStreamHandler classifies each line of `go <verb> vendor -v` stderr,
// reporting progress events and feeding modules.txt-shaped lines into a
// modulestxt.Stream. It's its own type so the classification logic — built
// from empirically testing real `go mod vendor -v` output across a large
// real-world graph, a tool directive, a remote replace (including its
// trailer line), a workspace, and failure cases — can be unit-tested
// independently of the process-execution plumbing in vendorModules.
type vendorStreamHandler struct {
	manifest      progress.Manifest
	reporter      progress.Reporter
	stream        modulestxt.Stream
	inVendorPhase bool
}

func newVendorStreamHandler(manifest progress.Manifest, reporter progress.Reporter) *vendorStreamHandler {
	h := &vendorStreamHandler{manifest: manifest, reporter: reporter}
	h.stream.Emit = h.vendored
	return h
}

// vendored reports one completed module entry from the stream.
func (h *vendorStreamHandler) vendored(m modulestxt.Module) {
	// The first modules.txt line to arrive marks the transition out of the
	// load phase (spawn + download + package-graph resolution, all silent
	// on stderr) into the vendor/copy phase.
	if !h.inVendorPhase {
		h.inVendorPhase = true
		h.reporter.Report(progress.PhaseChanged{Manifest: h.manifest, Phase: progress.Vendor})
	}
	h.reporter.Report(progress.Vendored{
		Manifest: h.manifest,
		Path:     m.Path,
		Version:  m.Version,
		Pkgs:     len(m.Packages),
	})
}

// line classifies and handles one line of stderr.
func (h *vendorStreamHandler) line(line string) {
	switch {
	case strings.HasPrefix(line, "go: downloading "):
		rest := strings.TrimPrefix(line, "go: downloading ")
		if path, version, ok := strings.Cut(rest, " "); ok {
			h.reporter.Report(progress.Downloading{Manifest: h.manifest, Path: path, Version: version})
			return
		}
		h.note(line)

	case strings.HasPrefix(line, "go:"), strings.HasPrefix(line, "warning:"):
		h.note(line)

	case looksLikeUnexpectedNoise(line):
		h.note(line)

	default:
		if err := h.stream.Line(line); err != nil {
			h.note(line)
		}
	}
}

func (h *vendorStreamHandler) note(line string) {
	h.reporter.Report(progress.Note{Manifest: h.manifest, Text: line})
}

// looksLikeUnexpectedNoise reports whether line is something other than a
// modules.txt header/annotation/package-path line. It exists because a
// multi-line "go: ..." message (confirmed with both an invalid-version
// go.mod parse error and a `go get` retraction warning) can have
// continuation lines that don't repeat the "go:" prefix — e.g. a
// tab-indented suggested command, or a bare "go.mod:N: ..." detail line.
// A real package path never contains whitespace, and a real header/
// annotation line always starts with "# " or "## ", so anything else that
// contains whitespace is treated as noise rather than risked as a
// misparsed package name.
func looksLikeUnexpectedNoise(line string) bool {
	if line == "" || strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "## ") {
		return false
	}
	return strings.ContainsAny(line, " \t")
}

// close flushes the final in-progress module entry, if any. It does not
// itself emit a lifecycle event — ResolveModule/ResolveWorkspace report
// Finished once the whole resolve is done.
func (h *vendorStreamHandler) close() {
	h.stream.Flush()
}

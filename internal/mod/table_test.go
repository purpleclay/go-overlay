package mod

import (
	"errors"
	"testing"

	"gotest.tools/v3/golden"
)

//nolint:revive
var errModuleNotFound = errors.New(`go: github.com/nonexistent/fakerepo@v1.0.0: reading github.com/nonexistent/fakerepo/go.mod at revision v1.0.0: git ls-remote -q origin in /Users/pthomas/go/pkg/mod/cache/vcs/52aff0ea21d8bd6eb4886c147731870e3188d46b62f672149e0462ee0612afe1: exit status 128:
	fatal: could not read Username for 'https://github.com': terminal prompts disabled
Confirm the import path was entered correctly.
If this is a private repository, see https://golang.org/doc/faq#git_https for additional information.`)

func TestRenderResultsTableForGoMod(t *testing.T) {
	results := []vendorResult{
		resultDrift("path/to/drift/go.mod", "sha256-A3tDBEG/zwPthZ+l5TxH8XpVY9FRw/iEQOtMryi9zXg=", "sha256-juCtIr7pvECB2svxXEKFvbdj/vqWUbu5EECe2te0RTI="),
		resultError("path/to/error/go.mod", errModuleNotFound),
		resultGenerated("path/to/generated/go.mod", 10),
		resultMissing("path/to/missing/go.mod"),
		resultNotFound("path/to/notfound/go.mod"),
		resultOK("path/to/ok/go.mod"),
		resultSkipped("path/to/skipped/go.mod"),
	}

	got := renderResultsTable(results)
	golden.Assert(t, got, "table_gomod.golden")
}

func TestRenderResultsTableForGoWork(t *testing.T) {
	results := []vendorResult{
		resultDrift("path/to/drift/go.work", "sha256-A3tDBEG/zwPthZ+l5TxH8XpVY9FRw/iEQOtMryi9zXg=", "sha256-juCtIr7pvECB2svxXEKFvbdj/vqWUbu5EECe2te0RTI="),
		resultError("path/to/error/go.work", errModuleNotFound),
		resultGenerated("path/to/generated/go.work", 10),
		resultMissing("path/to/missing/go.work"),
		resultNotFound("path/to/notfound/go.work"),
		resultOK("path/to/ok/go.work"),
		resultSkipped("path/to/skipped/go.work"),
	}

	got := renderResultsTable(results)
	golden.Assert(t, got, "table_gowork.golden")
}

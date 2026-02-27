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
		resultDriftDetected("path/to/drift/go.mod", []string{
			"hash: go.mod has changed\n      go.mod:        sha256-A3tDBEG/zwPthZ+l5TxH8XpVY9FRw/iEQOtMryi9zXg=\n      govendor.toml: sha256-juCtIr7pvECB2svxXEKFvbdj/vqWUbu5EECe2te0RTI=",
		}),
		resultDriftDetected("path/to/drift-layered/go.mod", []string{
			"govendor version mismatch: v1.0.0 → v2.0.0 (incompatible major version)",
			"hash: go.mod has changed\n      go.mod:        sha256-A3tDBEG/zwPthZ+l5TxH8XpVY9FRw/iEQOtMryi9zXg=\n      govendor.toml: sha256-juCtIr7pvECB2svxXEKFvbdj/vqWUbu5EECe2te0RTI=",
		}),
		resultError("path/to/error/go.mod", errModuleNotFound),
		resultGenerated("path/to/generated/go.mod", 10),
		resultMissing("path/to/missing/go.mod"),
		resultNotFound("path/to/notfound/go.mod"),
		resultOK("path/to/ok/go.mod"),
		resultSchemaMismatch("path/to/schema-mismatch/go.mod", 1, 2),
		resultSkipped("path/to/skipped/go.mod"),
		resultVersionWarning("path/to/warning/go.mod", []string{
			"govendor version mismatch: v0.9.0 → v0.10.0 (use --check --strict to enforce)",
		}),
	}

	got := renderResultsTable(results)
	golden.Assert(t, got, "table_gomod.golden")
}

func TestRenderResultsTableForGoWork(t *testing.T) {
	results := []vendorResult{
		resultDriftDetected("path/to/drift/go.work", []string{
			"hash: go.work has changed\n      go.work:       sha256-A3tDBEG/zwPthZ+l5TxH8XpVY9FRw/iEQOtMryi9zXg=\n      govendor.toml: sha256-juCtIr7pvECB2svxXEKFvbdj/vqWUbu5EECe2te0RTI=",
		}),
		resultDriftDetected("path/to/drift-layered/go.work", []string{
			"govendor version mismatch: v1.0.0 → v2.0.0 (incompatible major version)",
			"hash: go.work has changed\n      go.work:       sha256-A3tDBEG/zwPthZ+l5TxH8XpVY9FRw/iEQOtMryi9zXg=\n      govendor.toml: sha256-juCtIr7pvECB2svxXEKFvbdj/vqWUbu5EECe2te0RTI=",
		}),
		resultError("path/to/error/go.work", errModuleNotFound),
		resultGenerated("path/to/generated/go.work", 10),
		resultMissing("path/to/missing/go.work"),
		resultNotFound("path/to/notfound/go.work"),
		resultOK("path/to/ok/go.work"),
		resultSchemaMismatch("path/to/schema-mismatch/go.work", 1, 2),
		resultSkipped("path/to/skipped/go.work"),
		resultVersionWarning("path/to/warning/go.work", []string{
			"govendor version mismatch: v0.9.0 → v0.10.0 (use --check --strict to enforce)",
		}),
	}

	got := renderResultsTable(results)
	golden.Assert(t, got, "table_gowork.golden")
}

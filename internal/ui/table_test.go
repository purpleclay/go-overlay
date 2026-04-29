package ui_test

import (
	"errors"
	"testing"

	"github.com/purpleclay/go-overlay/internal/ui"
	"github.com/purpleclay/go-overlay/internal/vendor"
	"gotest.tools/v3/golden"
)

//nolint:revive
var errModuleNotFound = errors.New(`go: github.com/nonexistent/fakerepo@v1.0.0: reading github.com/nonexistent/fakerepo/go.mod at revision v1.0.0: git ls-remote -q origin in /Users/pthomas/go/pkg/mod/cache/vcs/52aff0ea21d8bd6eb4886c147731870e3188d46b62f672149e0462ee0612afe1: exit status 128:
	fatal: could not read Username for 'https://github.com': terminal prompts disabled
Confirm the import path was entered correctly.
If this is a private repository, see https://golang.org/doc/faq#git_https for additional information.`)

func TestRenderResultsTableForGoMod(t *testing.T) {
	results := []vendor.Result{
		{Path: "path/to/drift/go.mod", Status: vendor.StatusDrift, Message: "go.mod has changed, run 'govendor' to regenerate\n\n  go.mod:        sha256-A3tDBEG/zwPthZ+l5TxH8XpVY9FRw/iEQOtMryi9zXg=\n  govendor.toml: sha256-juCtIr7pvECB2svxXEKFvbdj/vqWUbu5EECe2te0RTI="},
		{Path: "path/to/error/go.mod", Status: vendor.StatusError, Message: errModuleNotFound.Error()},
		{Path: "path/to/generated/go.mod", Status: vendor.StatusGenerated, Message: "generated govendor.toml with 10 dependencies"},
		{Path: "path/to/missing/go.mod", Status: vendor.StatusMissing, Message: "govendor.toml not found, run govendor to generate"},
		{Path: "path/to/notfound/go.mod", Status: vendor.StatusError, Message: "go.mod does not exist, check path"},
		{Path: "path/to/ok/go.mod", Status: vendor.StatusOK, Message: "govendor.toml is up to date"},
		{Path: "path/to/schema-mismatch/go.mod", Status: vendor.StatusDrift, Message: "govendor.toml uses schema v1, current govendor requires schema v2 — run 'govendor' to regenerate"},
		{Path: "path/to/skipped/go.mod", Status: vendor.StatusSkipped, Message: "go.mod has no external dependencies"},
	}

	got := ui.RenderResultsTable(results)
	golden.Assert(t, got, "table_gomod.golden")
}

func TestRenderResultsTableForGoWork(t *testing.T) {
	results := []vendor.Result{
		{Path: "path/to/drift/go.work", Status: vendor.StatusDrift, Message: "go.work has changed, run 'govendor' to regenerate\n\n  go.work:       sha256-A3tDBEG/zwPthZ+l5TxH8XpVY9FRw/iEQOtMryi9zXg=\n  govendor.toml: sha256-juCtIr7pvECB2svxXEKFvbdj/vqWUbu5EECe2te0RTI="},
		{Path: "path/to/error/go.work", Status: vendor.StatusError, Message: errModuleNotFound.Error()},
		{Path: "path/to/generated/go.work", Status: vendor.StatusGenerated, Message: "generated govendor.toml with 10 dependencies"},
		{Path: "path/to/missing/go.work", Status: vendor.StatusMissing, Message: "govendor.toml not found, run govendor to generate"},
		{Path: "path/to/notfound/go.work", Status: vendor.StatusError, Message: "go.work does not exist, check path"},
		{Path: "path/to/ok/go.work", Status: vendor.StatusOK, Message: "govendor.toml is up to date"},
		{Path: "path/to/schema-mismatch/go.work", Status: vendor.StatusDrift, Message: "govendor.toml uses schema v1, current govendor requires schema v2 — run 'govendor' to regenerate"},
		{Path: "path/to/skipped/go.work", Status: vendor.StatusSkipped, Message: "go.work has no external dependencies"},
	}

	got := ui.RenderResultsTable(results)
	golden.Assert(t, got, "table_gowork.golden")
}

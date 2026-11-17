package ui_test

import (
	"bytes"
	"errors"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/purpleclay/go-overlay/internal/generate"
	"github.com/purpleclay/go-overlay/internal/ui"
	"gotest.tools/v3/golden"
)

//nolint:revive
var errModuleNotFound = errors.New(`go: github.com/nonexistent/fakerepo@v1.0.0: reading github.com/nonexistent/fakerepo/go.mod at revision v1.0.0: git ls-remote -q origin in /Users/pthomas/go/pkg/mod/cache/vcs/52aff0ea21d8bd6eb4886c147731870e3188d46b62f672149e0462ee0612afe1: exit status 128:
	fatal: could not read Username for 'https://github.com': terminal prompts disabled
Confirm the import path was entered correctly.
If this is a private repository, see https://golang.org/doc/faq#git_https for additional information.`)

func TestRenderResultsTableForGoMod(t *testing.T) {
	results := []generate.Result{
		{Path: "path/to/drift/go.mod", Status: generate.StatusDrift, Message: "go.mod has changed, run 'govendor' to regenerate"},
		{Path: "path/to/error/go.mod", Status: generate.StatusError, Message: errModuleNotFound.Error()},
		{Path: "path/to/generated/go.mod", Status: generate.StatusGenerated, Message: "generated govendor.toml with 10 dependencies"},
		{Path: "path/to/missing/go.mod", Status: generate.StatusMissing, Message: "govendor.toml not found, run govendor to generate"},
		{Path: "path/to/notfound/go.mod", Status: generate.StatusError, Message: "go.mod does not exist, check path"},
		{Path: "path/to/ok/go.mod", Status: generate.StatusOK, Message: "govendor.toml is up to date"},
		{Path: "path/to/schema-mismatch/go.mod", Status: generate.StatusDrift, Message: "govendor.toml uses schema v1, current govendor requires schema v3 — run 'govendor' to regenerate"},
	}

	got := renderPlain(t, results)
	golden.Assert(t, got, "table_gomod.golden")
}

func TestRenderResultsTableForGoWork(t *testing.T) {
	results := []generate.Result{
		{Path: "path/to/drift/go.work", Status: generate.StatusDrift, Message: "go.work has changed, run 'govendor' to regenerate"},
		{Path: "path/to/error/go.work", Status: generate.StatusError, Message: errModuleNotFound.Error()},
		{Path: "path/to/generated/go.work", Status: generate.StatusGenerated, Message: "generated govendor.toml with 10 dependencies"},
		{Path: "path/to/missing/go.work", Status: generate.StatusMissing, Message: "govendor.toml not found, run govendor to generate"},
		{Path: "path/to/notfound/go.work", Status: generate.StatusError, Message: "go.work does not exist, check path"},
		{Path: "path/to/ok/go.work", Status: generate.StatusOK, Message: "govendor.toml is up to date"},
		{Path: "path/to/schema-mismatch/go.work", Status: generate.StatusDrift, Message: "govendor.toml uses schema v1, current govendor requires schema v3 — run 'govendor' to regenerate"},
	}

	got := renderPlain(t, results)
	golden.Assert(t, got, "table_gowork.golden")
}

func renderPlain(t *testing.T, results []generate.Result) string {
	t.Helper()
	var buf bytes.Buffer
	if _, err := lipgloss.Fprint(&buf, ui.RenderResultsTable(results)); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

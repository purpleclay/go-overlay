package mod

import (
	"testing"

	"gotest.tools/v3/golden"
)

func TestRenderResultsTableForGoMod(t *testing.T) {
	results := []vendorResult{
		resultDrift("path/to/drift/go.mod"),
		resultError("path/to/error/go.mod", errVendorFailed),
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
		resultDrift("path/to/drift/go.work"),
		resultError("path/to/error/go.work", errVendorFailed),
		resultGenerated("path/to/generated/go.work", 10),
		resultMissing("path/to/missing/go.work"),
		resultNotFound("path/to/notfound/go.work"),
		resultOK("path/to/ok/go.work"),
		resultSkipped("path/to/skipped/go.work"),
	}

	got := renderResultsTable(results)
	golden.Assert(t, got, "table_gowork.golden")
}

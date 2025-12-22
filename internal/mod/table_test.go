package mod

import (
	"testing"

	"gotest.tools/v3/golden"
)

func TestRenderResultsTable(t *testing.T) {
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
	golden.Assert(t, got, "table.golden")
}

package progress

import (
	"errors"
	"testing"
)

func TestNopReporterDiscardsEverything(_ *testing.T) {
	var r NopReporter
	r.Report(Started{Manifest: "go.mod"})
	r.Report(Finished{Manifest: "go.mod", Err: errors.New("boom")})
}

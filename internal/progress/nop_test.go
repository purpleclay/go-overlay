package progress

import (
	"errors"
	"testing"
)

func TestNopReporterDiscardsEverything(_ *testing.T) {
	var r NopReporter
	r.Report(Started{Target: "go.mod"})
	r.Report(Finished{Target: "go.mod", Err: errors.New("boom")})
}

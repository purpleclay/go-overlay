package progress

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPhaseString(t *testing.T) {
	tests := []struct {
		phase Phase
		want  string
	}{
		{Load, "loading"},
		{Vendor, "vendoring"},
		{Hash, "hashing"},
		{Local, "resolving local replacements"},
		{Write, "writing manifest"},
		{Phase(99), "Phase(99)"},
		{Phase(-1), "Phase(-1)"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.phase.String())
	}
}

func TestEventSourceReportsItsManifest(t *testing.T) {
	events := []Event{
		Started{Manifest: "go.mod"},
		PhaseChanged{Manifest: "go.mod"},
		Downloading{Manifest: "go.mod"},
		Vendored{Manifest: "go.mod"},
		HashStarted{Manifest: "go.mod"},
		Hashed{Manifest: "go.mod"},
		Note{Manifest: "go.mod"},
		Finished{Manifest: "go.mod"},
	}

	for _, e := range events {
		assert.Equal(t, Manifest("go.mod"), e.Source())
	}
}

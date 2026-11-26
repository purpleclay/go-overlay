package progress

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventSourceReportsItsTarget(t *testing.T) {
	events := []Event{
		Started{Target: "go.mod"},
		Downloading{Target: "go.mod"},
		Vendored{Target: "go.mod"},
		HashStarted{Target: "go.mod"},
		Hashed{Target: "go.mod"},
		Note{Target: "go.mod"},
		Finished{Target: "go.mod"},
	}

	for _, e := range events {
		assert.Equal(t, Target("go.mod"), e.Source())
	}
}

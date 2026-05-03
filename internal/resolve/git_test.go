package resolve

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitTrackedFiles(t *testing.T) {
	output := `main.go
cmd/cli/root.go
cmd/cli/version.go
internal/server/serve.go`

	exec := &fakeExecutor{
		responses: map[string]string{
			"git ls-files": output,
		},
	}

	tests := []struct {
		name string
		dir  string
	}{
		{name: "clean path", dir: "/http-test"},
		{name: "trailing slash", dir: "/http-test/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracked, err := GitTrackedFiles(exec, tt.dir)
			require.NoError(t, err)

			// Regardless of input form, results are keyed by cleaned paths.
			clean := "/http-test"
			require.Len(t, tracked, 9)
			assert.Contains(t, tracked, clean)
			assert.Contains(t, tracked, filepath.Join(clean, "main.go"))
			assert.Contains(t, tracked, filepath.Join(clean, "cmd"))
			assert.Contains(t, tracked, filepath.Join(clean, "cmd/cli"))
			assert.Contains(t, tracked, filepath.Join(clean, "cmd/cli/root.go"))
			assert.Contains(t, tracked, filepath.Join(clean, "cmd/cli/version.go"))
			assert.Contains(t, tracked, filepath.Join(clean, "internal"))
			assert.Contains(t, tracked, filepath.Join(clean, "internal/server"))
			assert.Contains(t, tracked, filepath.Join(clean, "internal/server/serve.go"))
		})
	}
}

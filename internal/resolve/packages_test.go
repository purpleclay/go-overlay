package resolve

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePackagesByModule(t *testing.T) {
	output := `github.com/fatih/color	github.com/fatih/color
github.com/mattn/go-colorable	github.com/mattn/go-colorable
github.com/mattn/go-isatty	github.com/mattn/go-isatty
github.com/stretchr/testify	github.com/stretchr/testify/assert
github.com/stretchr/testify	github.com/stretchr/testify/require
golang.org/x/sys	golang.org/x/sys/unix
golang.org/x/sys	golang.org/x/sys/windows`

	result := ParsePackagesByModule(output)

	require.Len(t, result, 5)
	assert.Equal(t, []string{"github.com/fatih/color"}, result["github.com/fatih/color"])
	assert.Equal(t, []string{"github.com/mattn/go-colorable"}, result["github.com/mattn/go-colorable"])
	assert.Equal(t, []string{"github.com/mattn/go-isatty"}, result["github.com/mattn/go-isatty"])
	assert.Equal(t, []string{"github.com/stretchr/testify/assert", "github.com/stretchr/testify/require"}, result["github.com/stretchr/testify"])
	assert.Equal(t, []string{"golang.org/x/sys/unix", "golang.org/x/sys/windows"}, result["golang.org/x/sys"])
}

func TestParsePackagesByModuleSkipsLinesWithoutTab(t *testing.T) {
	output := `github.com/fatih/color	github.com/fatih/color
not-a-valid-line
github.com/mattn/go-isatty	github.com/mattn/go-isatty`

	result := ParsePackagesByModule(output)

	require.Len(t, result, 2)
	assert.Contains(t, result, "github.com/fatih/color")
	assert.Contains(t, result, "github.com/mattn/go-isatty")
}

func TestMergePackages(t *testing.T) {
	left := []string{"golang.org/x/sys/windows", "golang.org/x/sys/unix"}
	right := []string{"golang.org/x/sys/unix", "golang.org/x/sys/plan9"}

	expected := []string{"golang.org/x/sys/plan9", "golang.org/x/sys/unix", "golang.org/x/sys/windows"}
	assert.Equal(t, expected, MergePackages(left, right))
}

func TestMergePackagesNormalizesWhenLeftIsEmpty(t *testing.T) {
	right := []string{"golang.org/x/sys/windows", "golang.org/x/sys/unix", "golang.org/x/sys/unix"}

	expected := []string{"golang.org/x/sys/unix", "golang.org/x/sys/windows"}
	assert.Equal(t, expected, MergePackages(nil, right))
}

func TestMergePackagesNormalizesWhenRightIsEmpty(t *testing.T) {
	left := []string{"golang.org/x/sys/windows", "golang.org/x/sys/unix", "golang.org/x/sys/unix"}

	expected := []string{"golang.org/x/sys/unix", "golang.org/x/sys/windows"}
	assert.Equal(t, expected, MergePackages(left, nil))
}

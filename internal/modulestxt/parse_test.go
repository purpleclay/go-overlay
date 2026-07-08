package modulestxt_test

import (
	"os"
	"strings"
	"testing"

	"github.com/purpleclay/go-overlay/internal/modulestxt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustOpen(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { f.Close() })
	return f
}

func ptr[T any](v T) *T { return &v }

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []modulestxt.Module
	}{
		{
			name:    "SimpleModulesWithPackages",
			fixture: "testdata/simple.txt",
			want: []modulestxt.Module{
				{
					Path:      "charm.land/lipgloss/v2",
					Version:   "v2.0.3",
					Explicit:  true,
					GoVersion: "1.25.0",
					Packages:  []string{"charm.land/lipgloss/v2"},
				},
				{
					Path:      "github.com/charmbracelet/colorprofile",
					Version:   "v0.4.3",
					Explicit:  true,
					GoVersion: "1.25.0",
					Packages:  []string{"github.com/charmbracelet/colorprofile"},
				},
				{
					Path:      "github.com/charmbracelet/x/ansi",
					Version:   "v0.11.7",
					Explicit:  true,
					GoVersion: "1.24.2",
					Packages: []string{
						"github.com/charmbracelet/x/ansi",
						"github.com/charmbracelet/x/ansi/kitty",
						"github.com/charmbracelet/x/ansi/parser",
					},
				},
				{
					Path:      "golang.org/x/sys",
					Version:   "v0.44.0",
					Explicit:  true,
					GoVersion: "1.25.0",
					Packages:  []string{"golang.org/x/sys/unix", "golang.org/x/sys/windows"},
				},
			},
		},
		{
			name:    "RemoteReplacement",
			fixture: "testdata/remote-replace.txt",
			want: []modulestxt.Module{
				{
					Path:      "gopkg.in/ini.v1",
					Version:   "v1.67.0",
					Explicit:  true,
					GoVersion: "1.14",
					Packages:  []string{"gopkg.in/ini.v1"},
					Replace:   ptr(modulestxt.Replace{Path: "github.com/go-ini/ini", Version: "v1.67.0"}),
				},
				{
					Path:      "github.com/stretchr/testify",
					Version:   "v1.11.1",
					Explicit:  true,
					GoVersion: "1.20",
					Packages:  []string{"github.com/stretchr/testify"},
				},
			},
		},
		{
			// Exercises the wildcard local-replace form: "# path => ./dir" (no version).
			name:    "LocalReplacement",
			fixture: "testdata/local-replace.txt",
			want: []modulestxt.Module{
				{
					Path:      "github.com/go-overlay/examples/local-replaces/units",
					Version:   "",
					Explicit:  true,
					GoVersion: "1.26.3",
					Packages:  []string{"github.com/go-overlay/examples/local-replaces/units"},
					Replace:   ptr(modulestxt.Replace{Local: "./units"}),
				},
			},
		},
		{
			name:    "WorkspaceHeader",
			fixture: "testdata/workspace.txt",
			want: []modulestxt.Module{
				{
					Path:      "github.com/purpleclay/go-overlay/examples/go-workspace/mood",
					Version:   "v0.0.0",
					Explicit:  true,
					GoVersion: "1.25.4",
					Replace:   ptr(modulestxt.Replace{Local: "./mood"}),
				},
				{
					Path:      "golang.org/x/text",
					Version:   "v0.21.0",
					Explicit:  true,
					GoVersion: "1.23.0",
					Packages:  []string{"golang.org/x/text/language"},
				},
			},
		},
		{
			name:    "ModuleWithNoPackages",
			fixture: "testdata/no-packages.txt",
			want: []modulestxt.Module{
				{
					Path:      "github.com/foo/bar",
					Version:   "v1.0.0",
					Explicit:  true,
					GoVersion: "1.21",
				},
				{
					Path:      "github.com/foo/baz",
					Version:   "v2.0.0",
					Explicit:  true,
					GoVersion: "1.22",
					Packages:  []string{"github.com/foo/baz"},
				},
			},
		},
		{
			name:    "GoVersionOnlyAnnotation",
			fixture: "testdata/go-version-only.txt",
			want: []modulestxt.Module{
				{
					Path:      "github.com/foo/explicit",
					Version:   "v1.0.0",
					Explicit:  true,
					GoVersion: "1.21",
					Packages:  []string{"github.com/foo/explicit"},
				},
				{
					Path:      "github.com/foo/implicit",
					Version:   "v1.0.0",
					Explicit:  false,
					GoVersion: "1.20",
					Packages:  []string{"github.com/foo/implicit"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := mustOpen(t, tt.fixture)
			got, err := modulestxt.Parse(f)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseEmpty(t *testing.T) {
	got, err := modulestxt.Parse(strings.NewReader(""))
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestParseWorkspaceOnly(t *testing.T) {
	// A workspace modules.txt with only the header and no modules (edge case).
	got, err := modulestxt.Parse(strings.NewReader("## workspace\n"))
	require.NoError(t, err)
	assert.Empty(t, got)
}

package vendor_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/purpleclay/go-overlay/internal/mod"
	"github.com/purpleclay/go-overlay/internal/vendor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeGoMod(t *testing.T, dir, content string) *mod.GoModFile {
	t.Helper()
	path := filepath.Join(dir, mod.GoModFilename)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	goMod, err := mod.ParseGoModFile(path)
	require.NoError(t, err)
	return goMod
}

func writeGoWork(t *testing.T, dir, content string, members map[string]string) *mod.GoWorkFile {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, mod.GoWorkFilename), []byte(content), 0o644))
	for memberDir, goModContent := range members {
		fullDir := filepath.Join(dir, memberDir)
		require.NoError(t, os.MkdirAll(fullDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(fullDir, mod.GoModFilename), []byte(goModContent), 0o644))
	}
	goWork, err := mod.ParseGoWorkFile(filepath.Join(dir, mod.GoWorkFilename))
	require.NoError(t, err)
	return goWork
}

func TestIsDriftedGoMod(t *testing.T) {
	tests := []struct {
		name     string
		goMod    string
		existing *vendor.Manifest
		want     bool
	}{
		{
			name: "NoDrift",
			goMod: `module example.com/app
go 1.22
require github.com/foo/bar v1.0.0
`,
			existing: &vendor.Manifest{
				Schema: vendor.SchemaVersion,
				Mod:    map[string]mod.ModuleConfig{"github.com/foo/bar": {Version: "v1.0.0"}},
			},
			want: false,
		},
		{
			name: "RequireDrifted",
			goMod: `module example.com/app
go 1.22
require github.com/foo/bar v1.1.0
`,
			existing: &vendor.Manifest{
				Schema: vendor.SchemaVersion,
				Mod:    map[string]mod.ModuleConfig{"github.com/foo/bar": {Version: "v1.0.0"}},
			},
			want: true,
		},
		{
			name: "ReplacementAdded",
			goMod: `module example.com/app
go 1.22
require github.com/foo/bar v1.0.0
replace github.com/foo/bar => ../local
`,
			existing: &vendor.Manifest{
				Schema: vendor.SchemaVersion,
				Mod:    map[string]mod.ModuleConfig{"github.com/foo/bar": {Version: "v1.0.0"}},
			},
			want: true,
		},
		{
			name: "ReplacementRemoved",
			goMod: `module example.com/app
go 1.22
require github.com/foo/bar v1.0.0
`,
			existing: &vendor.Manifest{
				Schema: vendor.SchemaVersion,
				Mod:    map[string]mod.ModuleConfig{"github.com/foo/bar": {Version: "v1.0.0", Local: "../local"}},
			},
			want: true,
		},
		{
			name: "ExcludeAdded",
			goMod: `module example.com/app
go 1.22
require github.com/foo/bar v1.0.0
exclude github.com/baz/qux v1.0.0
`,
			existing: &vendor.Manifest{
				Schema: vendor.SchemaVersion,
				Mod:    map[string]mod.ModuleConfig{"github.com/foo/bar": {Version: "v1.0.0"}},
			},
			want: true,
		},
		{
			name: "ExcludeRemoved",
			goMod: `module example.com/app
go 1.22
require github.com/foo/bar v1.0.0
`,
			existing: &vendor.Manifest{
				Schema:  vendor.SchemaVersion,
				Exclude: map[string][]string{"github.com/baz/qux": {"v1.0.0"}},
				Mod:     map[string]mod.ModuleConfig{"github.com/foo/bar": {Version: "v1.0.0"}},
			},
			want: true,
		},
		{
			name: "ReplacementChanged",
			goMod: `module example.com/app
go 1.22
require github.com/foo/bar v1.0.0
replace github.com/foo/bar => ../new-local
`,
			existing: &vendor.Manifest{
				Schema: vendor.SchemaVersion,
				Mod:    map[string]mod.ModuleConfig{"github.com/foo/bar": {Version: "v1.0.0", Local: "../old-local"}},
			},
			want: true,
		},
		{
			name: "LocalToRemoteReplacement",
			goMod: `module example.com/app
go 1.22
require github.com/foo/bar v1.0.0
replace github.com/foo/bar => github.com/fork/bar v1.0.0
`,
			existing: &vendor.Manifest{
				Schema: vendor.SchemaVersion,
				Mod:    map[string]mod.ModuleConfig{"github.com/foo/bar": {Version: "v1.0.0", Local: "../local"}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goMod := writeGoMod(t, t.TempDir(), tt.goMod)
			got, err := vendor.IsDrifted(goMod, tt.existing)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsDriftedGoWork(t *testing.T) {
	tests := []struct {
		name     string
		goWork   string
		members  map[string]string
		existing *vendor.Manifest
		want     bool
	}{
		{
			name:   "NoDrift",
			goWork: "go 1.22\nuse ./api\n",
			members: map[string]string{
				"api": "module example.com/api\ngo 1.22\nrequire github.com/foo/bar v1.0.0\n",
			},
			existing: &vendor.Manifest{
				Schema: vendor.SchemaVersion,
				Mod:    map[string]mod.ModuleConfig{"github.com/foo/bar": {Version: "v1.0.0"}},
			},
			want: false,
		},
		{
			name:   "RequireDrifted",
			goWork: "go 1.22\nuse ./api\n",
			members: map[string]string{
				"api": "module example.com/api\ngo 1.22\nrequire github.com/foo/bar v1.1.0\n",
			},
			existing: &vendor.Manifest{
				Schema: vendor.SchemaVersion,
				Mod:    map[string]mod.ModuleConfig{"github.com/foo/bar": {Version: "v1.0.0"}},
			},
			want: true,
		},
		{
			name:   "WorkspaceReplacementAdded",
			goWork: "go 1.22\nuse ./api\n",
			members: map[string]string{
				"api": "module example.com/api\ngo 1.22\nrequire example.com/dep v1.0.0\nreplace example.com/dep => ../local\n",
			},
			existing: &vendor.Manifest{
				Schema: vendor.SchemaVersion,
				Mod:    map[string]mod.ModuleConfig{"example.com/dep": {Version: "v1.0.0"}},
			},
			want: true,
		},
		{
			name:   "WorkspaceExcludeAdded",
			goWork: "go 1.22\nuse ./api\n",
			members: map[string]string{
				"api": "module example.com/api\ngo 1.22\nrequire github.com/foo/bar v1.0.0\nexclude github.com/baz/qux v1.0.0\n",
			},
			existing: &vendor.Manifest{
				Schema: vendor.SchemaVersion,
				Mod:    map[string]mod.ModuleConfig{"github.com/foo/bar": {Version: "v1.0.0"}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goWork := writeGoWork(t, t.TempDir(), tt.goWork, tt.members)
			got, err := vendor.IsDrifted(goWork, tt.existing)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

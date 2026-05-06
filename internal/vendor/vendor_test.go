package vendor_test

import (
	"bytes"
	"path/filepath"
	"slices"
	"testing"

	"github.com/purpleclay/go-overlay/internal/mod"
	"github.com/purpleclay/go-overlay/internal/resolve"
	"github.com/purpleclay/go-overlay/internal/vendor"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/golden"
)

func TestVendor(t *testing.T) {
	tests := []struct {
		name             string
		dir              string
		includePlatforms []string
	}{
		{
			name: "simple",
			dir:  "testdata/simple",
		},
		{
			name:             "with-platforms",
			dir:              "testdata/with-platforms",
			includePlatforms: []string{"freebsd/amd64", "freebsd/arm64"},
		},
		{
			name: "local-replace",
			dir:  "testdata/local-replace",
		},
		{
			name: "remote-replace",
			dir:  "testdata/remote-replace",
		},
		{
			name: "tools-only",
			dir:  "testdata/tools-only",
		},
		{
			name: "with-excludes",
			dir:  "testdata/with-excludes",
		},
	}

	resolver := resolve.New(resolve.OSExecutor{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goModPath := filepath.Join(tt.dir, "go.mod")
			goMod, err := mod.ParseGoModFile(goModPath)
			require.NoError(t, err)

			platforms := mod.DefaultPlatforms()
			if len(tt.includePlatforms) > 0 {
				platforms = append(platforms, tt.includePlatforms...)
			}
			deps, err := resolver.ResolveModule(goMod, platforms)
			require.NoError(t, err)

			manifest := vendor.New(deps, tt.includePlatforms, nil, goMod.Excludes)

			var buf bytes.Buffer
			_, err = manifest.WriteTo(&buf)
			require.NoError(t, err)

			golden.Assert(t, buf.String(), tt.name+"/govendor.golden")
		})
	}
}

func TestVendorWorkspace(t *testing.T) {
	tests := []struct {
		name             string
		dir              string
		includePlatforms []string
	}{
		{
			name: "workspace",
			dir:  "testdata/workspace",
		},
		{
			name: "workspace-with-excludes",
			dir:  "testdata/workspace-with-excludes",
		},
		{
			name: "workspace-mvs-conflict",
			dir:  "testdata/workspace-mvs-conflict",
		},
	}

	resolver := resolve.New(resolve.OSExecutor{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goWorkPath := filepath.Join(tt.dir, "go.work")
			goWork, err := mod.ParseGoWorkFile(goWorkPath)
			require.NoError(t, err)

			platforms := mod.DefaultPlatforms()
			if len(tt.includePlatforms) > 0 {
				platforms = append(platforms, tt.includePlatforms...)
			}
			deps, err := resolver.ResolveWorkspace(goWork, platforms)
			require.NoError(t, err)

			members, err := goWork.ParseMembers()
			require.NoError(t, err)

			merged := make(map[string][]string)
			for _, m := range members {
				for path, versions := range m.Excludes {
					merged[path] = append(merged[path], versions...)
				}
			}
			for path, versions := range merged {
				slices.Sort(versions)
				merged[path] = slices.Compact(versions)
			}
			var excludes map[string][]string
			if len(merged) > 0 {
				excludes = merged
			}

			manifest := vendor.New(deps, tt.includePlatforms, goWork.WorkspaceConfig(), excludes)

			var buf bytes.Buffer
			_, err = manifest.WriteTo(&buf)
			require.NoError(t, err)

			golden.Assert(t, buf.String(), tt.name+"/govendor.golden")
		})
	}
}

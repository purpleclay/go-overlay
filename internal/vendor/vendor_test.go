package vendor_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/purpleclay/go-overlay/internal/mod"
	"github.com/purpleclay/go-overlay/internal/resolve"
	"github.com/purpleclay/go-overlay/internal/vendor"
	"github.com/stretchr/testify/assert"
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

			var tool mod.ToolConfig
			if len(goMod.Tools) > 0 {
				pkgToVersion := make(map[string]string)
				for _, dep := range deps {
					for _, pkg := range dep.Packages {
						pkgToVersion[pkg] = dep.Version
					}
				}
				tool = make(mod.ToolConfig, len(goMod.Tools))
				for _, pkg := range goMod.Tools {
					tool[pkg] = mod.ToolEntry{Version: pkgToVersion[pkg]}
				}
			}
			manifest := vendor.New(deps, tt.includePlatforms, nil, tool, goMod.Excludes)

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
		{
			name: "workspace-remote-replace",
			dir:  "testdata/workspace-remote-replace",
		},
		{
			name: "workspace-with-tools",
			dir:  "testdata/workspace-with-tools",
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

			var allTools []string
			merged := make(map[string][]string)
			for _, m := range members {
				allTools = append(allTools, m.Tools...)
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
			var tool mod.ToolConfig
			if len(allTools) > 0 {
				pkgToVersion := make(map[string]string)
				for _, dep := range deps {
					for _, pkg := range dep.Packages {
						pkgToVersion[pkg] = dep.Version
					}
				}
				slices.Sort(allTools)
				allTools = slices.Compact(allTools)
				tool = make(mod.ToolConfig, len(allTools))
				for _, pkg := range allTools {
					tool[pkg] = mod.ToolEntry{Version: pkgToVersion[pkg]}
				}
			}

			manifest := vendor.New(deps, tt.includePlatforms, goWork.WorkspaceConfig(), tool, excludes)

			var buf bytes.Buffer
			_, err = manifest.WriteTo(&buf)
			require.NoError(t, err)

			golden.Assert(t, buf.String(), tt.name+"/govendor.golden")
		})
	}
}

// fakeResolver satisfies vendor.Resolver and returns a fixed set of
// dependencies, ignoring the actual go.mod/go.work content. This lets
// processSource and VendorFiles be exercised without network calls.
type fakeResolver struct {
	deps []mod.ModuleConfig
}

func (f *fakeResolver) ResolveModule(_ *mod.GoModFile, _ []string) ([]mod.ModuleConfig, error) {
	return f.deps, nil
}

func (f *fakeResolver) ResolveWorkspace(_ *mod.GoWorkFile, _ []string) ([]mod.ModuleConfig, error) {
	return f.deps, nil
}

func setupModDir(t *testing.T, extra map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.26.0\n"), 0o644))
	for name, content := range extra {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	return dir
}

func vendorResults(t *testing.T, dir string, r vendor.Resolver, opts ...vendor.Option) []vendor.Result {
	t.Helper()
	opts = append([]vendor.Option{vendor.WithPaths(dir)}, opts...)
	v := vendor.NewVendor(r, opts...)
	results, _ := v.VendorFiles()
	return results
}

var chiDep = mod.ModuleConfig{
	Path:      "github.com/go-chi/chi/v5",
	Version:   "v5.2.2",
	Hash:      "sha256-F+KxLJNRQxkjQCDlJ72MT/YS8cybKPsLeWOjo6HqJHU=",
	GoVersion: "1.20",
	Packages:  []string{"github.com/go-chi/chi/v5"},
}

var chiDepWithMiddleware = mod.ModuleConfig{
	Path:      "github.com/go-chi/chi/v5",
	Version:   "v5.2.2",
	Hash:      "sha256-F+KxLJNRQxkjQCDlJ72MT/YS8cybKPsLeWOjo6HqJHU=",
	GoVersion: "1.20",
	Packages:  []string{"github.com/go-chi/chi/v5", "github.com/go-chi/chi/v5/middleware"},
}

func TestVendorWithCheck_MissingManifest(t *testing.T) {
	dir := setupModDir(t, nil)
	results := vendorResults(t, dir, &fakeResolver{}, vendor.WithDriftDetection())

	require.Len(t, results, 1)
	assert.Equal(t, vendor.StatusMissing, results[0].Status)
}

func TestVendorWithCheck_UnchangedManifest(t *testing.T) {
	dir := setupModDir(t, nil)
	vendorResults(t, dir, &fakeResolver{deps: []mod.ModuleConfig{chiDep}})

	results := vendorResults(t, dir, &fakeResolver{deps: []mod.ModuleConfig{chiDep}}, vendor.WithDriftDetection())
	require.Len(t, results, 1)
	assert.Equal(t, vendor.StatusOK, results[0].Status)
}

func TestVendorWithCheck_DriftPackageListChanged(t *testing.T) {
	dir := setupModDir(t, nil)
	vendorResults(t, dir, &fakeResolver{deps: []mod.ModuleConfig{chiDep}})

	results := vendorResults(t, dir, &fakeResolver{deps: []mod.ModuleConfig{chiDepWithMiddleware}}, vendor.WithDriftDetection())
	require.Len(t, results, 1)
	assert.Equal(t, vendor.StatusDrift, results[0].Status)
}

func TestVendorWithCheck_DriftVersionChanged(t *testing.T) {
	older := mod.ModuleConfig{
		Path:      "github.com/go-chi/chi/v5",
		Version:   "v5.2.1",
		Hash:      "sha256-oldHash=",
		GoVersion: "1.20",
		Packages:  []string{"github.com/go-chi/chi/v5"},
	}

	dir := setupModDir(t, nil)
	vendorResults(t, dir, &fakeResolver{deps: []mod.ModuleConfig{older}})

	results := vendorResults(t, dir, &fakeResolver{deps: []mod.ModuleConfig{chiDep}}, vendor.WithDriftDetection())
	require.Len(t, results, 1)
	assert.Equal(t, vendor.StatusDrift, results[0].Status)
}

func TestVendorWithCheck_DriftSchemaMismatch(t *testing.T) {
	dir := setupModDir(t, map[string]string{
		"govendor.toml": "# Generated by govendor. DO NOT EDIT.\n\nschema = 2\n\n[mod]\n",
	})

	results := vendorResults(t, dir, &fakeResolver{}, vendor.WithDriftDetection())
	require.Len(t, results, 1)
	assert.Equal(t, vendor.StatusDrift, results[0].Status)
	assert.Contains(t, results[0].Message, "schema")
}

func TestVendor_PackageListChangedRegenerates(t *testing.T) {
	dir := setupModDir(t, nil)
	vendorResults(t, dir, &fakeResolver{deps: []mod.ModuleConfig{chiDep}})

	results := vendorResults(t, dir, &fakeResolver{deps: []mod.ModuleConfig{chiDepWithMiddleware}})
	require.Len(t, results, 1)
	assert.Equal(t, vendor.StatusGenerated, results[0].Status)

	// Subsequent check confirms the manifest is now up to date.
	results = vendorResults(t, dir, &fakeResolver{deps: []mod.ModuleConfig{chiDepWithMiddleware}}, vendor.WithDriftDetection())
	require.Len(t, results, 1)
	assert.Equal(t, vendor.StatusOK, results[0].Status)
}

func TestVendor_UnchangedManifestSkipsWrite(t *testing.T) {
	dir := setupModDir(t, nil)
	vendorResults(t, dir, &fakeResolver{deps: []mod.ModuleConfig{chiDep}})

	results := vendorResults(t, dir, &fakeResolver{deps: []mod.ModuleConfig{chiDep}})
	require.Len(t, results, 1)
	assert.Equal(t, vendor.StatusOK, results[0].Status)
}

func TestVendorWithForce_RegeneratesUnchangedManifest(t *testing.T) {
	dir := setupModDir(t, nil)
	vendorResults(t, dir, &fakeResolver{deps: []mod.ModuleConfig{chiDep}})

	results := vendorResults(t, dir, &fakeResolver{deps: []mod.ModuleConfig{chiDep}}, vendor.WithForce())
	require.Len(t, results, 1)
	assert.Equal(t, vendor.StatusGenerated, results[0].Status)
}

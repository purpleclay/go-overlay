package govendor

import (
	"github.com/purpleclay/go-overlay/internal/mod"
	"github.com/spf13/cobra"
)

func Execute(build BuildDetails) error {
	var (
		check            bool
		recursive        bool
		depth            int
		includePlatforms []string
	)

	cmd := &cobra.Command{
		Use:   "govendor [paths...]",
		Short: "Generate a vendor manifest for building Go applications with Nix",
		Long: `Generate a govendor.toml manifest containing Go module metadata for use
with go-overlay's buildGoApplication Nix function.

The manifest includes module versions, NAR hashes, Go version requirements,
and package lists. This metadata enables Nix to build Go applications using
vendored dependencies without requiring nixpkgs' patched Go toolchain.

Supports both single modules (go.mod) and workspaces (go.work). When a go.work
file is detected, a unified manifest is generated containing dependencies from
all workspace modules.`,
		Example: `  # Generate vendor manifest for current directory
  govendor

  # Generate vendor manifest for specific paths
  govendor ./api ./web

  # Generate vendor manifest for a Go workspace (auto-detects go.work)
  govendor ./my-workspace

  # Recursively scan for go.mod files, limiting depth to 2 directories
  govendor --recursive --depth 2

  # Check if manifests have drifted and need updating
  govendor --check

  # Check specific paths for manifest drift
  govendor --check ./api ./web

  # Recursively check for manifest drift, limiting depth to 2 directories
  govendor --check --recursive --depth 2

  # Include additional platforms for cross-compilation
  govendor --include-platform=freebsd/amd64 --include-platform=openbsd/amd64`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			var opts []mod.VendorOption

			if len(args) > 0 {
				opts = append(opts, mod.WithPaths(args...))
			}

			if check {
				opts = append(opts, mod.WithDriftDetection())
			}

			if recursive {
				opts = append(opts, mod.WithRecursive(depth))
			}

			if len(includePlatforms) > 0 {
				if err := mod.ValidatePlatforms(includePlatforms); err != nil {
					return err
				}
				opts = append(opts, mod.WithIncludePlatforms(includePlatforms))
			}

			v := mod.NewVendor(opts...)
			return v.VendorFiles()
		},
	}

	cmd.Flags().BoolVarP(&check, "check", "c", false, "check if manifests have drifted and need updating")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "recursively scan for go.mod files (ignores go.work)")
	cmd.Flags().IntVarP(&depth, "depth", "d", 0, "limit directory traversal depth (0 = unlimited, requires --recursive)")
	cmd.Flags().StringArrayVar(&includePlatforms, "include-platform", nil, "extend platform list for dependency resolution (e.g., freebsd/amd64)")

	cmd.Version = build.Version
	cmd.SetVersionTemplate(build.String())
	return cmd.Execute()
}

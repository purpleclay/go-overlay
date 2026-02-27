package govendor

import (
	"fmt"

	"github.com/purpleclay/go-overlay/internal/mod"
	"github.com/purpleclay/x/cli"
	"github.com/purpleclay/x/theme"
	"github.com/spf13/cobra"
)

func Execute(version cli.VersionInfo) error {
	var (
		check            bool
		force            bool
		recursive        bool
		workspace        bool
		strict           bool
		depth            int
		includePlatforms []string
	)

	cmd := &cobra.Command{
		Use:   "govendor [PATHS...]",
		Short: "Generate a vendor manifest for building Go applications with Nix",
		Long: `
		Generate a govendor.toml manifest containing Go module metadata for use
		with go-overlay's buildGoApplication Nix function.

		The manifest includes module versions, NAR hashes, Go version requirements,
		and package lists. This metadata enables Nix to build Go applications using
		vendored dependencies without requiring nixpkgs' patched Go toolchain.

		Supports both single modules (go.mod) and workspaces (go.work). When a go.work
		file is detected, a unified manifest is generated containing dependencies from
		all workspace modules. As go.work files are typically added to a .gitignore file,
		the workspace is reconstructed from the manifest when go.work is not present.
		`,
		Example: `
		# Generate vendor manifest for current directory
		govendor

		# Generate vendor manifest for specific paths
		govendor ./api ./web

		# Recursively scan for go.mod files, limiting depth to 2 directories
		govendor --recursive --depth 2

		# Check if manifests have drifted and need updating
		govendor --check

		# Check specific paths for manifest drift
		govendor --check ./api ./web

		# Recursively check for manifest drift, limiting depth to 2 directories
		govendor --check --recursive --depth 2

		# Reverse scan from a submodule path for a workspace manifest (govendor.toml)
		govendor --check --workspace theme/go.mod

		# Treat all check warnings as errors, failing validation
		govendor --check --strict

		# Include additional platforms for cross-compilation
		govendor --include-platform=freebsd/amd64 --include-platform=openbsd/amd64

		# Force regeneration of govendor.toml, bypassing hash check
		govendor --force
		`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if workspace && !check {
				return fmt.Errorf("--workspace requires --check")
			}

			if strict && !check {
				return fmt.Errorf("--strict requires --check")
			}

			var opts []mod.VendorOption

			if len(args) > 0 {
				opts = append(opts, mod.WithPaths(args...))
			}

			if check {
				opts = append(opts, mod.WithDriftDetection())
			}

			if force {
				opts = append(opts, mod.WithForce())
			}

			if recursive {
				opts = append(opts, mod.WithRecursive(depth))
			}

			if workspace {
				opts = append(opts, mod.WithWorkspace())
			}

			if len(includePlatforms) > 0 {
				if err := mod.ValidatePlatforms(includePlatforms); err != nil {
					return err
				}
				opts = append(opts, mod.WithIncludePlatforms(includePlatforms))
			}

			if version.Version != "dev" && version.Version != "" {
				opts = append(opts, mod.WithVendoredVersion(version.Version))
			}

			if strict {
				opts = append(opts, mod.WithStrict())
			}

			v := mod.NewVendor(opts...)
			return v.VendorFiles()
		},
	}

	cmd.Flags().BoolVarP(&check, "check", "c", false, "check if manifests have drifted and need updating")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "force regeneration of govendor.toml, bypassing hash check")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "recursively scan for go.mod files (ignores go.work)")
	cmd.Flags().BoolVarP(&workspace, "workspace", "w", false, "reverse scan from a submodule path for a govendor.toml containing a workspace manifest (requires --check)")
	cmd.Flags().BoolVar(&strict, "strict", false, "treat all check warnings as errors, failing validation (requires --check)")
	cmd.Flags().IntVarP(&depth, "depth", "d", 0, "limit directory traversal depth (0 = unlimited)")
	cmd.Flags().StringArrayVar(&includePlatforms, "include-platform", nil, "extend platform list for dependency resolution (e.g., freebsd/amd64)")
	cmd.MarkFlagsMutuallyExclusive("recursive", "workspace")
	cmd.MarkFlagsMutuallyExclusive("force", "check")
	cmd.MarkFlagsMutuallyExclusive("strict", "force")

	return cli.Execute(cmd,
		cli.WithVersionFlag(version),
		cli.WithTheme(theme.PurpleClayCLI()),
	)
}

package govendor

import (
	"errors"
	"fmt"
	"io"

	"github.com/purpleclay/go-overlay/internal/resolve"
	"github.com/purpleclay/go-overlay/internal/ui"
	"github.com/purpleclay/go-overlay/internal/vendor"
	"github.com/purpleclay/x/cli"
	"github.com/purpleclay/x/theme"
	"github.com/spf13/cobra"
)

// Exit code convention, matching gofmt / terraform fmt -check:
//
//	0: all manifests up to date / generated
//	1: drift or missing manifest detected (--check)
//	2: execution error (toolchain failure, parse error, bad flags)
//
// Mixed results report the most severe code.
const (
	exitOK    = 0
	exitDrift = 1
	exitError = 2
)

// resultsExitCode returns the most severe exit code implied by results.
// Callers only invoke this when VendorFiles has already returned a non-nil
// error, which it only does when at least one result is a failure — the
// exitError fallback below guards that invariant rather than mislabelling an
// unexpected all-success set as drift.
func resultsExitCode(results []vendor.Result) int {
	sawDrift := false
	for _, r := range results {
		if r.Status == vendor.StatusError {
			return exitError
		}
		if r.Status == vendor.StatusDrift || r.Status == vendor.StatusMissing {
			sawDrift = true
		}
	}
	if sawDrift {
		return exitDrift
	}
	return exitError
}

func Execute(version cli.VersionInfo, args []string) (int, error) {
	var (
		check            bool
		recursive        bool
		workspace        bool
		depth            int
		includePlatforms []string
		tableRendered    bool
		exitCode         int
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

		`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if workspace && !check {
				return fmt.Errorf("--workspace requires --check")
			}

			var opts []vendor.Option

			if len(args) > 0 {
				opts = append(opts, vendor.WithPaths(args...))
			}

			if check {
				opts = append(opts, vendor.WithDriftDetection())
			}

			if recursive {
				opts = append(opts, vendor.WithRecursive(depth))
			}

			if workspace {
				opts = append(opts, vendor.WithWorkspace())
			}

			if len(includePlatforms) > 0 {
				return fmt.Errorf("--include-platform is no longer supported: resolution is now platform-independent (AnyTags) and covers all platforms unconditionally; remove the flag and regenerate your manifest")
			}

			resolver := resolve.New(resolve.OSExecutor{})
			v := vendor.NewVendor(resolver, opts...)
			results, err := v.VendorFiles(cmd.Context())
			if len(results) > 0 {
				tableRendered = true
				fmt.Fprintln(cmd.OutOrStdout(), ui.RenderResultsTable(results))
			}
			if err != nil {
				exitCode = resultsExitCode(results)
			}
			return err
		},
	}

	cmd.Flags().BoolVarP(&check, "check", "c", false, "check if manifests have drifted and need updating")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "recursively scan for go.mod files (ignores go.work)")
	cmd.Flags().BoolVarP(&workspace, "workspace", "w", false, "reverse scan from a submodule path for a govendor.toml containing a workspace manifest (requires --check)")
	cmd.Flags().IntVarP(&depth, "depth", "d", 0, "limit directory traversal depth (0 = unlimited)")
	cmd.Flags().StringArrayVar(&includePlatforms, "include-platform", nil, "removed: resolution is now platform-independent, this flag will error if set")
	cmd.MarkFlagsMutuallyExclusive("recursive", "workspace")
	cmd.SetArgs(args)

	cli.ExitCodes(
		cmd,
		cli.ExitCode{Code: exitOK, Desc: "manifests up to date/generated"},
		cli.ExitCode{Code: exitDrift, Desc: "drift or missing manifest detected (--check)"},
		cli.ExitCode{Code: exitError, Desc: "execution error (toolchain failure, parse error, bad flags)"},
	)

	err := cli.Execute(
		cmd,
		cli.WithVersionFlag(version),
		cli.WithTheme(theme.PurpleClayCLI()),
		cli.WithErrorHandler(func(w io.Writer, t cli.Theme, err error) {
			// The results table already reports per-result failures; printing
			// the generic sentinel underneath it adds nothing new.
			if tableRendered && errors.Is(err, vendor.ErrVendorFailed) {
				return
			}
			cli.DefaultErrorHandler(w, t, err)
		}),
	)

	// RunE may never set exitCode (e.g. cobra's own flag-parsing errors occur
	// before RunE runs), or may exit early via a bare error return (e.g. bad
	// flag combinations). Either way, an error with no severity already
	// assigned is an execution error, not a drift signal.
	if err != nil && exitCode == exitOK {
		exitCode = exitError
	}

	return exitCode, err
}

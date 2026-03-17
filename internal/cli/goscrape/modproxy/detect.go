package modproxy

import (
	"fmt"

	"github.com/purpleclay/go-overlay/internal/proxy"
	"github.com/purpleclay/go-overlay/internal/version"
	"github.com/spf13/cobra"
)

func newDetectCmd() *cobra.Command {
	var (
		prefix string
		all    bool
	)

	cmd := &cobra.Command{
		Use:   "detect MODULE",
		Short: "Detect the latest version of a Go module from the module proxy",
		Long: `
		Queries the Go module proxy (https://proxy.golang.org) and detects versions
		of the given module. By default, returns the latest semver-tagged version.
		Use --all to list all available versions. An optional prefix flag can restrict
		results to a specific version line.
		`,
		Example: `
		# Detect the latest version of govulncheck
		goscrape mod-proxy detect golang.org/x/vuln

		# Detect the latest 1.0.x version
		goscrape mod-proxy detect golang.org/x/vuln --prefix 1.0

		# List all versions of govulncheck
		goscrape mod-proxy detect golang.org/x/vuln --all

		# List all 1.1.x versions
		goscrape mod-proxy detect golang.org/x/vuln --prefix 1.1 --all
		`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			versions, err := proxy.ListVersions(args[0], prefix)
			if err != nil {
				return err
			}

			if all {
				for _, v := range versions {
					fmt.Fprintln(cmd.OutOrStdout(), v)
				}
				return nil
			}

			latest, err := version.Latest(versions, args[0])
			if err != nil {
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), latest)
			return nil
		},
	}

	cmd.Flags().StringVarP(&prefix, "prefix", "p", "", "filter versions by prefix (e.g. 1.1)")
	cmd.Flags().BoolVar(&all, "all", false, "list all available versions instead of just the latest")
	return cmd
}

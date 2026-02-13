package goscrapeproxy

import (
	"fmt"

	"github.com/purpleclay/go-overlay/internal/proxy"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var prefix string

	cmd := &cobra.Command{
		Use:   "list [MODULE]",
		Short: "List available versions of a Go module from the module proxy",
		Long: `
		Queries the Go module proxy (https://proxy.golang.org) and lists all
		semver-tagged versions of the given module. Pseudo-versions are excluded.
		An optional prefix flag can filter results to a specific version line.
		`,
		Example: `
		# List all versions of govulncheck
		goscrapeproxy list golang.org/x/vuln

		# List all versions of govulncheck starting with semver prefix 1.1
		goscrapeproxy list golang.org/x/vuln --prefix 1.1
		`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			versions, err := proxy.ListVersions(args[0], prefix)
			if err != nil {
				return err
			}

			for _, v := range versions {
				fmt.Fprintln(cmd.OutOrStdout(), v)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&prefix, "prefix", "p", "", "filter versions by semver prefix (e.g. 1.1)")
	return cmd
}

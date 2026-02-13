package goscrapeproxy

import (
	"fmt"

	"github.com/purpleclay/go-overlay/internal/proxy"
	"github.com/spf13/cobra"
)

func newDetectCmd() *cobra.Command {
	var prefix string

	cmd := &cobra.Command{
		Use:   "detect [MODULE]",
		Short: "Detect the latest version of a Go module from the module proxy",
		Long: `
		Queries the Go module proxy (https://proxy.golang.org) and detects the
		latest semver-tagged version of the given module. An optional prefix flag
		can restrict detection to a specific version line.
		`,
		Example: `
		# Detect the latest version of govulncheck
  		goscrapeproxy detect golang.org/x/vuln

    	# Detect the latest 1.0.x version
     	goscrapeproxy detect golang.org/x/vuln --prefix 1.0
      	`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			versions, err := proxy.ListVersions(args[0], prefix)
			if err != nil {
				return err
			}

			if len(versions) == 0 {
				return fmt.Errorf("no versions found for module %s", args[0])
			}

			fmt.Fprint(cmd.OutOrStdout(), versions[len(versions)-1])
			return nil
		},
	}

	cmd.Flags().StringVarP(&prefix, "prefix", "p", "", "filter versions by prefix (e.g. 1.1)")
	return cmd
}

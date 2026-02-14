package goscrapeproxy

import (
	"github.com/purpleclay/x/cli"
	"github.com/purpleclay/x/theme"
	"github.com/spf13/cobra"
)

func Execute(version cli.VersionInfo) error {
	cmd := &cobra.Command{
		Use:           "goscrapeproxy",
		Short:         "Tools for scraping Go tool releases from the Go module proxy and generating Nix manifests",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(newDetectCmd(), newGenerateCmd(), newListCmd())

	return cli.Execute(cmd,
		cli.WithVersionFlag(version),
		cli.WithTheme(theme.PurpleClayCLI()),
	)
}

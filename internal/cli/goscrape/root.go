package goscrape

import (
	"github.com/purpleclay/go-overlay/internal/cli/goscrape/godev"
	"github.com/purpleclay/go-overlay/internal/cli/goscrape/modproxy"
	"github.com/purpleclay/x/cli"
	"github.com/purpleclay/x/theme"
	"github.com/spf13/cobra"
)

func Execute(version cli.VersionInfo) error {
	cmd := &cobra.Command{
		Use:   "goscrape",
		Short: "Tools for scraping Go releases and generating Nix manifests",
		Long: `
		goscrape provides utilities for working with Go releases and Go tool
		releases, including detecting available versions and generating Nix
		manifest files.
		`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(godev.NewCmd(), modproxy.NewCmd())

	return cli.Execute(cmd,
		cli.WithVersionFlag(version),
		cli.WithTheme(theme.PurpleClayCLI()),
	)
}

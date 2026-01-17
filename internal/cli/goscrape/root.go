package goscrape

import (
	"context"

	"github.com/purpleclay/go-overlay/internal/scrape"
	"github.com/purpleclay/x/cli"
	"github.com/purpleclay/x/theme"
	"github.com/spf13/cobra"
)

type contextKey string

const pageDataKey contextKey = "pageData"

func Execute(version cli.VersionInfo) error {
	cmd := &cobra.Command{
		Use:   "goscrape",
		Short: "Tools for scraping Go releases and generating Nix manifests",
		Long: `
		goscrape provides utilities for working with Go releases from https://go.dev/dl/
		including listing available versions, detecting latest releases, and generating
		Nix manifest files with SHA256 hashes for each platform.
		`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			page, err := scrape.FetchDownloadPage()
			if err != nil {
				return err
			}
			ctx := context.WithValue(cmd.Context(), pageDataKey, page)
			cmd.SetContext(ctx)
			return nil
		},
	}

	cmd.AddCommand(newGenerateCmd(), newDetectCmd(), newListCmd())

	return cli.Execute(cmd,
		cli.WithVersionFlag(version),
		cli.WithTheme(theme.PurpleClayCLI()),
	)
}

package goscrape

import (
	"context"
	"io"

	"github.com/purpleclay/go-overlay/internal/scrape"
	"github.com/spf13/cobra"
)

type contextKey string

const pageDataKey contextKey = "pageData"

func Execute(out io.Writer) error {
	cmd := &cobra.Command{
		Use:   "goscrape",
		Short: "Tools for scraping Go releases and generating Nix manifests",
		Long: `go-scrape provides utilities for working with Go releases from https://go.dev/dl/
including listing available versions, detecting latest releases, and generating
Nix manifest files with SHA256 hashes for each platform.`,
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

	cmd.AddCommand(newGenerateCmd(out), newDetectCmd(out), newListCmd(out))
	return cmd.Execute()
}

package godev

import (
	"context"

	"github.com/purpleclay/go-overlay/internal/scrape"
	"github.com/spf13/cobra"
)

type contextKey string

const pageDataKey contextKey = "pageData"

// NewCmd returns the go-dev subcommand group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "go-dev",
		Short: "Tools for working with Go releases from https://go.dev/dl/",
		Long: `
		Provides commands for interacting with Go releases hosted on the Golang
		website (https://go.dev/dl/), including detecting the latest versions
		and generating Nix manifests with SHA256 hashes for each platform.
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

	cmd.AddCommand(newDetectCmd(), newGenerateCmd())
	return cmd
}

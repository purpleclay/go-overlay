package modproxy

import "github.com/spf13/cobra"

// NewCmd returns the mod-proxy subcommand group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mod-proxy",
		Short: "Tools for working with Go module releases from the Go module proxy",
		Long: `
		Provides commands for interacting with Go tool releases hosted on the
		Go module proxy (https://proxy.golang.org), including detecting the
		latest versions and generating Nix manifests with NAR hashes.
		`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(newDetectCmd(), newGenerateCmd())
	return cmd
}

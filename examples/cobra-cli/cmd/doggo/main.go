package main

import (
	"fmt"
	"os"

	"github.com/purpleclay/go-overlay/examples/cobra-cli/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:   "doggo",
		Short: "Find out what dog you deserve",
	}

	root.AddCommand(versionCmd(), suggestCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of doggo",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("doggo %s\n", version)
		},
	}
}

func suggestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "suggest",
		Short: "Find out what dog you deserve",
		Run: func(cmd *cobra.Command, args []string) {
			p := tea.NewProgram(tui.New(), tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		},
	}
}

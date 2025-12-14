package cmd

import (
	"fmt"
	"go-scrape/internal/scrape"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func listVersions(page, prefix string) ([]string, error) {
	versions := make(map[string]bool)
	rem := page

	for {
		var out string
		var ver string
		var err error

		rem, out, err = scrape.Href(prefix)(rem)
		if err != nil {
			break
		}

		_, ver, err = scrape.GoVersion()(strings.TrimPrefix(out, "/dl/"))
		if err != nil {
			continue
		}

		versions[ver] = true
	}

	result := make([]string, 0, len(versions))
	for v := range versions {
		result = append(result, v)
	}
	sort.Strings(result)

	return result, nil
}

func newListCmd(out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "list [prefix]",
		Short: "List all available Go versions",
		Long: `Scrapes the Golang website (https://go.dev/dl/) and lists all available
Go versions. Optionally filter by a version prefix to show only versions
matching a specific release line.`,
		Example: `  # List all available Go versions
  $ go-scrape list

  # List all Go 1.21.x versions
  $ go-scrape list 1.21

  # List all Go 1.20.x versions
  $ go-scrape list go1.20`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			page, ok := cmd.Context().Value(pageDataKey).(string)
			if !ok {
				return fmt.Errorf("failed to retrieve page data from context")
			}

			var prefix string
			if len(args) == 1 {
				prefix = args[0]
			}

			versions, err := listVersions(page, prefix)
			if err != nil {
				return err
			}

			for _, v := range versions {
				fmt.Fprintf(out, "%s\n", v)
			}

			return nil
		},
	}
}

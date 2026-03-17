package godev

import (
	"fmt"
	"sort"
	"strings"

	"github.com/purpleclay/go-overlay/internal/scrape"
	"github.com/spf13/cobra"
)

func detectVersion(page, ver string) (string, error) {
	if ver == "" {
		return scrape.FetchLatestVersion()
	}

	return parseVersion(page, ver)
}

func parseVersion(page, ver string) (string, error) {
	_, ext, err := scrape.Href(ver)(page)
	if err != nil {
		return "", fmt.Errorf("version %s not found on https://go.dev/dl/", ver)
	}

	var rel string
	_, rel, err = scrape.GoVersion()(strings.TrimPrefix(ext, "/dl/"))
	if err != nil {
		return "", fmt.Errorf("failed to parse version from download link: %w", err)
	}
	return rel, nil
}

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

func newDetectCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "detect [PREFIX]",
		Short: "Detect the latest Go release or list all available versions",
		Long: `
		Scrapes the Golang website (https://go.dev/dl/) to detect Go versions.
		By default, returns the latest version. Use --all to list all available
		versions. Optionally provide a version prefix to filter results to a
		specific release line.
		`,
		Example: `
		# Detect the latest Go version
		goscrape go-dev detect

		# Detect the latest patch version of Go 1.21
		goscrape go-dev detect 1.21

		# List all available Go versions
		goscrape go-dev detect --all

		# List all Go 1.21.x versions
		goscrape go-dev detect 1.21 --all
		`,
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

			if all {
				versions, err := listVersions(page, prefix)
				if err != nil {
					return err
				}
				for _, v := range versions {
					fmt.Fprintln(cmd.OutOrStdout(), v)
				}
				return nil
			}

			ver, err := detectVersion(page, prefix)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s", ver)
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "list all available versions instead of just the latest")
	return cmd
}

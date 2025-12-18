package goscrape

import (
	"fmt"
	"io"
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

func newDetectCmd(out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "detect [prefix]",
		Short: "Detect the latest version of a Go release",
		Long: `Scrapes the Golang website (https://go.dev/dl/) to detect the latest version
of a Golang release. Optionally provide a version prefix to find the latest
patch version of a specific release.`,
		Example: `  # Detect the latest Go version
  $ go-scrape detect

  # Detect the latest patch version of Go 1.21
  $ go-scrape detect 1.21`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			page, ok := cmd.Context().Value(pageDataKey).(string)
			if !ok {
				return fmt.Errorf("failed to retrieve page data from context")
			}

			var ver string
			var err error

			if len(args) == 1 {
				ver = args[0]
			}

			latestVersion, err := detectVersion(page, ver)
			if err != nil {
				return err
			}

			fmt.Fprintf(out, "%s", latestVersion)
			return nil
		},
	}
}

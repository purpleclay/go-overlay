package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

const (
	goDownloadURL = "https://go.dev/dl/"
	goVersionURL  = "https://go.dev/VERSION?m=text"
)

var httpClient = &http.Client{
	Timeout: 5 * time.Second,
}

func fetch(url string) (string, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code returned (%d) when querying: %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func fetchDownloadPage() (string, error) {
	return fetch(goDownloadURL)
}

func fetchLatestVersion() (string, error) {
	data, err := fetch(goVersionURL)
	if err != nil {
		return "", err
	}

	_, version, err := goVersion()(data)
	if err != nil {
		return "", err
	}
	return version, nil
}

type contextKey string

const pageDataKey contextKey = "pageData"

func execute(out io.Writer) error {
	cmd := &cobra.Command{
		Use:   "go-scrape",
		Short: "Tools for scraping Go releases and generating Nix manifests",
		Long: `go-scrape provides utilities for working with Go releases from https://go.dev/dl/
including listing available versions, detecting latest releases, and generating
Nix manifest files with SHA256 hashes for each platform.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			page, err := fetchDownloadPage()
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

func main() {
	if err := execute(os.Stdout); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

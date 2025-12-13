package scrape

import (
	"fmt"
	"io"
	"net/http"
	"time"
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

func FetchDownloadPage() (string, error) {
	return fetch(goDownloadURL)
}

func FetchLatestVersion() (string, error) {
	data, err := fetch(goVersionURL)
	if err != nil {
		return "", err
	}

	_, version, err := GoVersion()(data)
	if err != nil {
		return "", err
	}
	return version, nil
}

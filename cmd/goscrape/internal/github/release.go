package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	gitHubAPIBase = "https://api.github.com"
	goRepo        = "golang/go"
)

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// tagRef represents the GitHub API response for a tag reference
type tagRef struct {
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

// commit represents the GitHub API response for a commit
type commit struct {
	Commit struct {
		Committer struct {
			Date time.Time `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

func fetch(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "go-scrape")

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return io.ReadAll(resp.Body)
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("GitHub API authentication failed: GITHUB_TOKEN is invalid or expired")
	case http.StatusForbidden:
		return nil, fmt.Errorf("GitHub API rate limit exceeded: set GITHUB_TOKEN for higher limits")
	default:
		return nil, fmt.Errorf("unexpected status code %d from GitHub API: %s", resp.StatusCode, url)
	}
}

func FetchReleaseDate(version string) (time.Time, error) {
	tag := version
	if !strings.HasPrefix(version, "go") {
		tag = "go" + version
	}

	tagURL := fmt.Sprintf("%s/repos/%s/git/refs/tags/%s", gitHubAPIBase, goRepo, tag)
	tagData, err := fetch(tagURL)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to fetch tag ref for %s: %w", tag, err)
	}

	var ref tagRef
	if err := json.Unmarshal(tagData, &ref); err != nil {
		return time.Time{}, fmt.Errorf("failed to parse tag ref response: %w", err)
	}

	commitURL := fmt.Sprintf("%s/repos/%s/commits/%s", gitHubAPIBase, goRepo, ref.Object.SHA)
	commitData, err := fetch(commitURL)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to fetch commit for %s: %w", ref.Object.SHA, err)
	}

	var c commit
	if err := json.Unmarshal(commitData, &c); err != nil {
		return time.Time{}, fmt.Errorf("failed to parse commit response: %w", err)
	}

	return c.Commit.Committer.Date, nil
}

package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

const baseURL = "https://proxy.golang.org"

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
		return "", fmt.Errorf("unexpected status code (%d) when querying: %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func ListVersions(module, prefix string) ([]string, error) {
	url := fmt.Sprintf("%s/%s/@v/list", baseURL, module)
	data, err := fetch(url)
	if err != nil {
		return nil, err
	}

	var versions []string
	for line := range strings.SplitSeq(strings.TrimSpace(data), "\n") {
		v := strings.TrimSpace(line)
		if v == "" {
			continue
		}

		if !semver.IsValid(v) {
			continue
		}

		if prefix != "" && !matchesPrefix(v, prefix) {
			continue
		}

		versions = append(versions, v)
	}

	semver.Sort(versions)
	return versions, nil
}

func matchesPrefix(version, prefix string) bool {
	trimmed := strings.TrimPrefix(version, "v")
	return strings.HasPrefix(trimmed, prefix)
}

//nolint:tagliatelle
type ModuleInfo struct {
	Version string    `json:"Version"`
	Time    time.Time `json:"Time"`
}

func FetchInfo(module, version string) (*ModuleInfo, error) {
	url := fmt.Sprintf("%s/%s/@v/%s.info", baseURL, module, version)
	data, err := fetch(url)
	if err != nil {
		return nil, err
	}

	var info ModuleInfo
	if err := json.Unmarshal([]byte(data), &info); err != nil {
		return nil, fmt.Errorf("failed to parse .info response for %s@%s: %w", module, version, err)
	}

	return &info, nil
}

func FetchGoMod(module, version string) (string, error) {
	url := fmt.Sprintf("%s/%s/@v/%s.mod", baseURL, module, version)
	return fetch(url)
}

func ParseGoDirective(raw string) (string, error) {
	f, err := modfile.ParseLax("go.mod", []byte(raw), nil)
	if err != nil {
		return "", fmt.Errorf("failed to parse go.mod content: %w", err)
	}

	if f.Go == nil {
		return "", fmt.Errorf("go.mod does not contain a go directive")
	}

	return f.Go.Version, nil
}

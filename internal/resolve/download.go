package resolve

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ModuleDownload is one entry from `go mod download -json`.
//
//nolint:tagliatelle
type ModuleDownload struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
	Dir     string `json:"Dir"`
	GoMod   string `json:"GoMod"`
	Error   string `json:"Error"`
}

// ParseDownloadOutput parses the JSON stream output of `go mod download -json`.
// Each JSON object is a separate module download result.
func ParseDownloadOutput(out string) ([]ModuleDownload, error) {
	var downloads []ModuleDownload
	dec := json.NewDecoder(strings.NewReader(out))
	for {
		var meta ModuleDownload
		if err := dec.Decode(&meta); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if meta.Error != "" {
			return nil, fmt.Errorf("failed to download %s@%s: %s", meta.Path, meta.Version, meta.Error)
		}
		downloads = append(downloads, meta)
	}
	return downloads, nil
}

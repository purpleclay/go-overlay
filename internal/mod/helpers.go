package mod

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nix-community/go-nix/pkg/nar"
	"golang.org/x/mod/modfile"
)

func parseDownloadOutput(out string) ([]goModuleDownload, error) {
	var downloads []goModuleDownload
	dec := json.NewDecoder(strings.NewReader(out))
	for {
		var meta goModuleDownload
		if err := dec.Decode(&meta); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		downloads = append(downloads, meta)
	}
	return downloads, nil
}

func resolveModuleFromDownload(meta goModuleDownload, pkgsByMod map[string][]string) (GoModule, error) {
	h := sha256.New()
	if err := nar.DumpPathFilter(h, meta.Dir, func(path string, _ nar.NodeType) bool {
		return strings.ToLower(filepath.Base(path)) != ".ds_store"
	}); err != nil {
		return GoModule{}, err
	}

	digest := h.Sum(nil)
	hash := "sha256-" + base64.StdEncoding.EncodeToString(digest)

	var goVersion string
	if meta.GoMod != "" {
		if modData, err := os.ReadFile(meta.GoMod); err == nil {
			if modFile, err := modfile.Parse(meta.GoMod, modData, nil); err == nil && modFile.Go != nil {
				goVersion = modFile.Go.Version
			}
		}
	}

	return GoModule{
		Path:      meta.Path,
		Version:   meta.Version,
		Packages:  pkgsByMod[meta.Path],
		Hash:      hash,
		GoVersion: goVersion,
	}, nil
}

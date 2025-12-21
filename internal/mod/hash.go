package mod

import (
	"github.com/BurntSushi/toml"
)

func extractHash(data []byte) (string, error) {
	var manifest struct {
		Hash string `toml:"hash"`
	}
	if err := toml.Unmarshal(data, &manifest); err != nil {
		return "", err
	}
	return manifest.Hash, nil
}

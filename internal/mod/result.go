package mod

import (
	"fmt"
	"path/filepath"

	"github.com/purpleclay/go-overlay/internal/vendor"
)

func fileType(path string) string {
	if filepath.Base(path) == goWorkFile {
		return "go.work"
	}
	return "go.mod"
}

func resultOK(path string) vendor.Result {
	return vendor.Result{Path: path, Status: vendor.StatusOK, Message: "govendor.toml is up to date"}
}

func resultGenerated(path string, count int) vendor.Result {
	return vendor.Result{Path: path, Status: vendor.StatusGenerated, Message: fmt.Sprintf("generated govendor.toml with %d dependencies", count)}
}

func resultDrift(path, currentHash, manifestHash string) vendor.Result {
	ft := fileType(path)
	msg := fmt.Sprintf("%s has changed, run 'govendor' to regenerate\n\n  %-14s %s\n  %-14s %s", ft, ft+":", currentHash, "govendor.toml:", manifestHash)
	return vendor.Result{Path: path, Status: vendor.StatusDrift, Message: msg}
}

func resultSchemaMismatch(path string, manifestSchema, currentSchema int) vendor.Result {
	msg := fmt.Sprintf("govendor.toml uses schema v%d, current govendor requires schema v%d — run 'govendor' to regenerate", manifestSchema, currentSchema)
	return vendor.Result{Path: path, Status: vendor.StatusDrift, Message: msg}
}

func resultMissing(path string) vendor.Result {
	return vendor.Result{Path: path, Status: vendor.StatusMissing, Message: "govendor.toml not found, run govendor to generate"}
}

func resultSkipped(path string) vendor.Result {
	ft := fileType(path)
	return vendor.Result{Path: path, Status: vendor.StatusSkipped, Message: fmt.Sprintf("%s has no external dependencies", ft)}
}

func resultError(path string, err error) vendor.Result {
	return vendor.Result{Path: path, Status: vendor.StatusError, Message: err.Error()}
}

func resultNotFound(path string) vendor.Result {
	ft := fileType(path)
	return vendor.Result{Path: path, Status: vendor.StatusError, Message: fmt.Sprintf("%s does not exist, check path", ft)}
}

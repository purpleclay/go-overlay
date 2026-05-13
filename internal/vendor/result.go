package vendor

import (
	"fmt"
	"path/filepath"

	"github.com/purpleclay/go-overlay/internal/mod"
)

// Status represents the outcome of processing a single go.mod or go.work.
type Status string

const (
	StatusOK        Status = "ok"
	StatusGenerated Status = "generated"
	StatusDrift     Status = "drift"
	StatusMissing   Status = "missing"
	StatusError     Status = "error"
)

func (s Status) IsSuccess() bool {
	return s == StatusOK || s == StatusGenerated
}

func (s Status) IsFailure() bool {
	return s == StatusDrift || s == StatusMissing || s == StatusError
}

// Result captures the outcome of processing a single file.
type Result struct {
	Path    string
	Status  Status
	Message string
}

func fileType(path string) string {
	if filepath.Base(path) == mod.GoWorkFilename {
		return mod.GoWorkFilename
	}
	return mod.GoModFilename
}

func resultOK(path string) Result {
	return Result{Path: path, Status: StatusOK, Message: "govendor.toml is up to date"}
}

func resultGenerated(path string, count int) Result {
	return Result{Path: path, Status: StatusGenerated, Message: fmt.Sprintf("generated govendor.toml with %d dependencies", count)}
}

func resultDrift(path string) Result {
	ft := fileType(path)
	msg := fmt.Sprintf("%s has changed, run 'govendor' to regenerate", ft)
	return Result{Path: path, Status: StatusDrift, Message: msg}
}

func resultSchemaMismatch(path string, manifestSchema, currentSchema int) Result {
	msg := fmt.Sprintf("govendor.toml uses schema v%d, current govendor requires schema v%d — run 'govendor' to regenerate", manifestSchema, currentSchema)
	return Result{Path: path, Status: StatusDrift, Message: msg}
}

func resultMissing(path string) Result {
	return Result{Path: path, Status: StatusMissing, Message: "govendor.toml not found, run govendor to generate"}
}

func resultError(path string, err error) Result {
	return Result{Path: path, Status: StatusError, Message: err.Error()}
}

func resultNotFound(path string) Result {
	ft := fileType(path)
	return Result{Path: path, Status: StatusError, Message: fmt.Sprintf("%s does not exist, check path", ft)}
}

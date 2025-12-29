package mod

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
)

type vendorStatus string

const (
	statusOK        vendorStatus = "ok"
	statusGenerated vendorStatus = "generated"
	statusDrift     vendorStatus = "drift"
	statusMissing   vendorStatus = "missing"
	statusSkipped   vendorStatus = "skipped"
	statusError     vendorStatus = "error"
)

var (
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

func (s vendorStatus) IsSuccess() bool {
	return s == statusOK || s == statusGenerated || s == statusSkipped
}

func (s vendorStatus) IsFailure() bool {
	return s == statusDrift || s == statusMissing || s == statusError
}

func (s vendorStatus) Symbol() string {
	switch s {
	case statusOK, statusGenerated:
		return greenStyle.Render("✓")
	case statusDrift, statusMissing, statusError:
		return redStyle.Render("✗")
	case statusSkipped:
		return yellowStyle.Render("○")
	default:
		return " "
	}
}

func (s vendorStatus) Render() string {
	switch s {
	case statusOK, statusGenerated:
		return greenStyle.Render(string(s))
	case statusDrift, statusMissing, statusError:
		return redStyle.Render(string(s))
	case statusSkipped:
		return yellowStyle.Render(string(s))
	default:
		return string(s)
	}
}

type vendorResult struct {
	path    string
	status  vendorStatus
	message string
}

func fileType(path string) string {
	if filepath.Base(path) == goWorkFile {
		return "go.work"
	}
	return "go.mod"
}

func resultOK(path string) vendorResult {
	return vendorResult{path: path, status: statusOK, message: "govendor.toml is up to date"}
}

func resultGenerated(path string, count int) vendorResult {
	return vendorResult{path: path, status: statusGenerated, message: fmt.Sprintf("generated govendor.toml with %d dependencies", count)}
}

func resultDrift(path string) vendorResult {
	ft := fileType(path)
	return vendorResult{path: path, status: statusDrift, message: fmt.Sprintf("%s has changed, regenerate govendor.toml", ft)}
}

func resultMissing(path string) vendorResult {
	return vendorResult{path: path, status: statusMissing, message: "govendor.toml not found, run govendor to generate"}
}

func resultSkipped(path string) vendorResult {
	ft := fileType(path)
	return vendorResult{path: path, status: statusSkipped, message: fmt.Sprintf("%s has no external dependencies", ft)}
}

func resultError(path string, err error) vendorResult {
	return vendorResult{path: path, status: statusError, message: err.Error()}
}

func resultNotFound(path string) vendorResult {
	ft := fileType(path)
	return vendorResult{path: path, status: statusError, message: fmt.Sprintf("%s does not exist, check path", ft)}
}

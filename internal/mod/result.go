package mod

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type vendorStatus string

const (
	statusOK        vendorStatus = "ok"
	statusGenerated vendorStatus = "generated"
	statusDrift     vendorStatus = "drift"
	statusMissing   vendorStatus = "missing"
	statusSkipped   vendorStatus = "skipped"
	statusWarning   vendorStatus = "warning"
	statusError     vendorStatus = "error"
)

var (
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

func (s vendorStatus) IsSuccess() bool {
	return s == statusOK || s == statusGenerated || s == statusSkipped || s == statusWarning
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
	case statusSkipped, statusWarning:
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
	case statusSkipped, statusWarning:
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

func resultSchemaMismatch(path string, manifestSchema, currentSchema int) vendorResult {
	msg := fmt.Sprintf("govendor.toml uses schema v%d, current govendor requires schema v%d — run 'govendor' to regenerate", manifestSchema, currentSchema)
	return vendorResult{path: path, status: statusDrift, message: msg}
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

func buildReasonTree(header string, reasons []string) string {
	var sb strings.Builder
	sb.WriteString(header)
	for i, r := range reasons {
		sb.WriteString("\n  ")
		if i == len(reasons)-1 {
			sb.WriteString("└── ")
		} else {
			sb.WriteString("├── ")
		}
		sb.WriteString(r)
	}
	return sb.String()
}

func resultDriftDetected(path string, reasons []string) vendorResult {
	msg := buildReasonTree("drift detected, run 'govendor' to regenerate", reasons)
	return vendorResult{path: path, status: statusDrift, message: msg}
}

func resultVersionWarning(path string, reasons []string) vendorResult {
	msg := buildReasonTree("govendor.toml is up to date", reasons)
	return vendorResult{path: path, status: statusWarning, message: msg}
}

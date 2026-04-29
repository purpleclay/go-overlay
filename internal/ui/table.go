package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/muesli/reflow/wordwrap"
	"github.com/purpleclay/go-overlay/internal/vendor"
)

var (
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

	headerStyle  = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	cellStyle    = lipgloss.NewStyle().Padding(0, 1)
	messageStyle = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("8"))
	borderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

const messageWrapWidth = 90

// StatusSymbol returns a coloured symbol for the given status.
func StatusSymbol(s vendor.Status) string {
	switch s {
	case vendor.StatusOK, vendor.StatusGenerated:
		return greenStyle.Render("✓")
	case vendor.StatusDrift, vendor.StatusMissing, vendor.StatusError:
		return redStyle.Render("✗")
	case vendor.StatusSkipped:
		return yellowStyle.Render("○")
	default:
		return " "
	}
}

// StatusLabel returns a coloured label for the given status.
func StatusLabel(s vendor.Status) string {
	switch s {
	case vendor.StatusOK, vendor.StatusGenerated:
		return greenStyle.Render(string(s))
	case vendor.StatusDrift, vendor.StatusMissing, vendor.StatusError:
		return redStyle.Render(string(s))
	case vendor.StatusSkipped:
		return yellowStyle.Render(string(s))
	default:
		return string(s)
	}
}

// RenderResultsTable formats a slice of results as a bordered terminal table
// with coloured status indicators.
func RenderResultsTable(results []vendor.Result) string {
	var rows [][]string
	for _, r := range results {
		status := StatusSymbol(r.Status) + " " + StatusLabel(r.Status)
		message := wordwrap.String(r.Message, messageWrapWidth)
		rows = append(rows, []string{r.Path, status, message})
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(borderStyle).
		Headers("File", "Status", "Message").
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			if col == 2 {
				return messageStyle
			}
			return cellStyle
		}).
		Rows(rows...)

	return t.Render()
}

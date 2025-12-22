package mod

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var (
	headerStyle  = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	cellStyle    = lipgloss.NewStyle().Padding(0, 1)
	messageStyle = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("8"))
	borderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func renderResultsTable(results []vendorResult) string {
	var rows [][]string
	for _, r := range results {
		status := r.status.Symbol() + " " + r.status.Render()
		rows = append(rows, []string{r.path, status, r.message})
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(borderStyle).
		Headers("GoMod File", "Status", "Message").
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

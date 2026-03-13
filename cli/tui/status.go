package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// StatusRow holds one row of the status table.
type StatusRow struct {
	Server     string
	Service    string
	Healthy    bool
	StatusCode int
}

var (
	styleStatusHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	styleStatusOK     = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	styleStatusFail   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleStatusCol    = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
)

// RenderStatus prints the status table to stdout.
func RenderStatus(rows []StatusRow) {
	fmt.Println(styleStatusHeader.Render("┌─ kaptan status ───────────────────────────────┐"))
	fmt.Printf("│ %-16s %-22s %-8s│\n",
		styleStatusCol.Render("SERVER"),
		styleStatusCol.Render("SERVICE"),
		styleStatusCol.Render("HEALTH"),
	)
	fmt.Println("│" + strings.Repeat("─", 50) + "│")

	for _, r := range rows {
		var health string
		if r.Healthy {
			health = styleStatusOK.Render(fmt.Sprintf("✓ %d", r.StatusCode))
		} else {
			code := r.StatusCode
			if code == 0 {
				health = styleStatusFail.Render("✗ down")
			} else {
				health = styleStatusFail.Render(fmt.Sprintf("✗ %d", code))
			}
		}
		fmt.Printf("│ %-16s %-22s %-8s│\n", r.Server, r.Service, health)
	}

	fmt.Println("└" + strings.Repeat("─", 50) + "┘")
}

package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	agentv1 "github.com/yourusername/kaptan/proto/agent/v1"
)

var (
	styleGraphHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	styleEdgeOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	styleEdgeErr     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleEdgeNode    = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
)

type graphModel struct {
	serverName string
	edges      []*agentv1.GraphEdge
}

func (m graphModel) Init() tea.Cmd { return nil }

func (m graphModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m graphModel) View() string {
	var b strings.Builder
	b.WriteString(styleGraphHeader.Render(fmt.Sprintf("kaptan graph — %s", m.serverName)) + "\n\n")

	// group edges by source
	fromMap := map[string][]*agentv1.GraphEdge{}
	var order []string
	seen := map[string]bool{}
	for _, e := range m.edges {
		if !seen[e.From] {
			order = append(order, e.From)
			seen[e.From] = true
		}
		fromMap[e.From] = append(fromMap[e.From], e)
	}

	for _, from := range order {
		b.WriteString(styleEdgeNode.Render(from) + "\n")
		edges := fromMap[from]
		for i, e := range edges {
			connector := "    ├─"
			if i == len(edges)-1 {
				connector = "    └─"
			}
			var codeStr string
			if e.StatusCode >= 400 {
				codeStr = styleEdgeErr.Render(fmt.Sprintf("[%d]", e.StatusCode))
			} else {
				codeStr = styleEdgeOK.Render(fmt.Sprintf("[%d]", e.StatusCode))
			}
			line := fmt.Sprintf("%s%s──► %s", connector, codeStr, e.To)
			if e.ErrorCount > 0 {
				line += styleEdgeErr.Render(fmt.Sprintf("  ← %derr/5min", e.ErrorCount))
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("\n(q to quit)")

	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).
		Render(b.String())
}

// RunGraph displays the dependency graph TUI.
func RunGraph(serverName string, edges []*agentv1.GraphEdge) error {
	if len(edges) == 0 {
		fmt.Println("No dependency edges found in log.")
		return nil
	}
	m := graphModel{serverName: serverName, edges: edges}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

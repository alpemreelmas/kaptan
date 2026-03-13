package tui

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	agentv1 "github.com/yourusername/kaptan/proto/agent/v1"
)

// rePhase matches "[N/M] description..." lines from deploy scripts.
var rePhase = regexp.MustCompile(`^\[(\d+)/(\d+)\]\s+(.+)`)

// --- styles ---

var (
	styleHeader  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	styleLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	styleOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).SetString("✓")
	styleRunning = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).SetString("● running")
	stylePending = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).SetString("·")
	styleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).SetString("✗")
	styleLog     = lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
	styleBorder  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

// --- messages ---

type execEventMsg struct{ ev *agentv1.ExecEvent }
type streamDoneMsg struct{ err error }

// --- model ---

type phase struct {
	index   int
	total   int
	label   string
	done    bool
	failed  bool
}

type deployModel struct {
	service    string
	server     string
	serverHost string
	script     string
	stream     agentv1.AgentService_DeployClient

	phases     []phase
	currentPhase int
	logs       []string
	done       bool
	exitCode   int32
	err        error

	width int
}

// --- init ---

func (m deployModel) Init() tea.Cmd {
	return recvEvent(m.stream)
}

// --- update ---

func (m deployModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width

	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case execEventMsg:
		ev := msg.ev
		if ev.Done {
			m.done = true
			m.exitCode = ev.ExitCode
			// mark last running phase
			if m.currentPhase < len(m.phases) {
				if ev.ExitCode != 0 {
					m.phases[m.currentPhase].failed = true
				} else {
					m.phases[m.currentPhase].done = true
				}
			}
			return m, tea.Quit
		}

		if ev.Line != "" {
			// check if it's a phase line
			if match := rePhase.FindStringSubmatch(ev.Line); match != nil {
				label := match[3]
				idx := len(m.phases)
				// mark previous phase done
				if idx > 0 && !m.phases[idx-1].done {
					m.phases[idx-1].done = true
				}
				m.phases = append(m.phases, phase{
					index: idx,
					label: strings.TrimRight(label, "."),
				})
				m.currentPhase = idx
			} else {
				// add to log (cap at 20 lines)
				m.logs = append(m.logs, ev.Line)
				if len(m.logs) > 20 {
					m.logs = m.logs[len(m.logs)-20:]
				}
			}
		}
		return m, recvEvent(m.stream)

	case streamDoneMsg:
		m.done = true
		if msg.err != nil {
			m.err = msg.err
		}
		return m, tea.Quit
	}

	return m, nil
}

// --- view ---

func (m deployModel) View() string {
	w := m.width
	if w == 0 {
		w = 80
	}

	var b strings.Builder
	b.WriteString(styleHeader.Render("kaptan deploy") + "\n\n")

	b.WriteString(styleLabel.Render("  Service   ") + m.service + "\n")
	b.WriteString(styleLabel.Render("  Server    ") + m.server + " (" + m.serverHost + ")\n")
	b.WriteString(styleLabel.Render("  Script    ") + m.script + "\n\n")

	for i, ph := range m.phases {
		var status string
		switch {
		case ph.failed:
			status = styleError.String()
		case ph.done:
			status = styleOK.String()
		case i == m.currentPhase && !m.done:
			status = styleRunning.String()
		default:
			status = stylePending.String()
		}
		label := fmt.Sprintf("  [%d/%d] %-30s", i+1, len(m.phases), ph.label)
		b.WriteString(styleLabel.Render(label) + " " + status + "\n")
	}

	if len(m.logs) > 0 {
		b.WriteString("\n")
		b.WriteString(styleLabel.Render("  ─── log ───\n"))
		for _, l := range m.logs {
			b.WriteString(styleLog.Render("  "+l) + "\n")
		}
	}

	if m.done {
		b.WriteString("\n")
		if m.err != nil {
			b.WriteString(styleError.String() + " " + m.err.Error() + "\n")
		} else if m.exitCode != 0 {
			b.WriteString(styleError.String() + fmt.Sprintf(" exited with code %d\n", m.exitCode))
		} else {
			b.WriteString(styleOK.String() + " deploy complete\n")
		}
	}

	return styleBorder.Width(w - 2).Render(b.String())
}

// --- runner ---

// RunDeploy starts the deploy TUI and streams events until done.
func RunDeploy(serverName, serverHost, scriptPath string, stream agentv1.AgentService_DeployClient) error {
	// try to get service name from project config
	service := scriptPath

	m := deployModel{
		service:    service,
		server:     serverName,
		serverHost: serverHost,
		script:     scriptPath,
		stream:     stream,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	dm := finalModel.(deployModel)
	if dm.err != nil {
		return dm.err
	}
	if dm.exitCode != 0 {
		return fmt.Errorf("deploy failed with exit code %d", dm.exitCode)
	}
	return nil
}

// recvEvent is a tea.Cmd that reads the next gRPC event.
func recvEvent(stream agentv1.AgentService_DeployClient) tea.Cmd {
	return func() tea.Msg {
		ev, err := stream.Recv()
		if err == io.EOF {
			return streamDoneMsg{}
		}
		if err != nil {
			return streamDoneMsg{err: err}
		}
		return execEventMsg{ev: ev}
	}
}

// spinner tick (unused but kept for future animation)
var _ = time.Second

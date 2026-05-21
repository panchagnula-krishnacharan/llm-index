package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/krishna/llm-index/internal/db"
)

var (
	appStyle     = lipgloss.NewStyle().Padding(1, 2)
	titleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFDF5")).Background(lipgloss.Color("#6C50FF")).Padding(0, 1).MarginBottom(1)
	statusStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	previewStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#6C50FF")).Padding(1, 2)
)

type viewState int

const (
	viewTable viewState = iota
	viewPreview
)

type model struct {
	pool     *pgxpool.Pool
	table    table.Model
	viewport viewport.Model
	state    viewState
	sessions []db.Session
	width    int
	height   int
}

func Run(pool *pgxpool.Pool) {
	sessions, err := db.ListSessions(pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	columns := []table.Column{
		{Title: "Time", Width: 14},
		{Title: "Type", Width: 10},
		{Title: "Title", Width: 44},
		{Title: "Model", Width: 18},
		{Title: "Msgs", Width: 5},
	}

	rows := make([]table.Row, len(sessions))
	for i, s := range sessions {
		rows[i] = table.Row{
			s.UpdatedAt.Format("Jan 02 15:04"),
			s.Source,
			truncate(s.Title, 42),
			s.Model,
			fmt.Sprintf("%d", s.MessageCount),
		}
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	st := table.DefaultStyles()
	st.Header = st.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#6C50FF")).
		BorderBottom(true).
		Bold(true)
	st.Selected = st.Selected.
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(lipgloss.Color("#6C50FF")).
		Bold(false)
	t.SetStyles(st)

	m := model{
		pool:     pool,
		table:    t,
		sessions: sessions,
		state:    viewTable,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetHeight(msg.Height - 8)
		m.viewport = viewport.New(msg.Width-4, msg.Height-6)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.state == viewPreview {
				m.state = viewTable
				return m, nil
			}
			return m, tea.Quit

		case "enter":
			if m.state == viewTable {
				idx := m.table.Cursor()
				if idx >= 0 && idx < len(m.sessions) {
					return m, resumeSession(m.sessions[idx])
				}
			}

		case "p":
			if m.state == viewTable {
				idx := m.table.Cursor()
				if idx >= 0 && idx < len(m.sessions) {
					m.state = viewPreview
					msgs, _ := db.GetMessages(m.pool, m.sessions[idx].ID)
					m.viewport.SetContent(renderMessages(msgs))
					return m, nil
				}
			}

		case "esc":
			if m.state == viewPreview {
				m.state = viewTable
				return m, nil
			}
		}
	}

	if m.state == viewTable {
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.state == viewPreview {
		header := titleStyle.Render("Preview") + "  " + statusStyle.Render("esc: back")
		return appStyle.Render(header + "\n" + previewStyle.Width(m.width-8).Render(m.viewport.View()))
	}

	header := titleStyle.Render("LLM Sessions")
	help := statusStyle.Render("enter: resume · p: preview · q: quit")
	return appStyle.Render(header + "\n\n" + m.table.View() + "\n\n" + help)
}

func renderMessages(msgs []db.Message) string {
	var sb strings.Builder
	for _, msg := range msgs {
		role := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#6C50FF")).Render(msg.Role)
		sb.WriteString(role + "\n")
		content := msg.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		sb.WriteString(content + "\n\n")
	}
	return sb.String()
}

func resumeSession(s db.Session) tea.Cmd {
	return tea.Sequence(
		tea.ExecProcess(buildResumeCmd(s), func(err error) tea.Msg {
			return tea.Quit()
		}),
	)
}

func buildResumeCmd(s db.Session) *exec.Cmd {
	parts := strings.Fields(s.ResumeCmd)
	if len(parts) == 0 {
		return exec.Command("echo", "no resume command configured")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	if s.Directory != "" {
		cmd.Dir = s.Directory
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

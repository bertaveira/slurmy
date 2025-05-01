package main

import (
	"fmt"
	"os"
	"slurmy/slurm"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var docStyle = lipgloss.NewStyle()

type model struct {
	jobs   list.Model
	width  int
	height int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate list size (approx 1/3 width)
		listWidth := m.width / 3
		// Ensure border fits: width must be at least 2
		if listWidth < 2 {
			listWidth = 2
		}

		// Create the list style temporarily to get its vertical border size
		listStyle := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true).
			Width(listWidth - 2) // Match the width used in View
		listVerticalMargin := listStyle.GetVerticalFrameSize()

		h, v := docStyle.GetFrameSize() // Use docStyle frame size
		// Adjust height for docStyle frame AND list border
		m.jobs.SetSize(listWidth-h, m.height-v-listVerticalMargin)
	}
	var cmd tea.Cmd
	m.jobs, cmd = m.jobs.Update(msg)
	return m, cmd
}

func (m model) View() string {
	listView := m.jobs.View()

	var selectedJob slurm.JobInfo
	if item, ok := m.jobs.SelectedItem().(slurm.JobInfo); ok {
		selectedJob = item
	}

	details := fmt.Sprintf(
		"Job ID: %s\nName: %s\nStatus: %s\n",
		selectedJob.JobID,
		selectedJob.JobName,
		selectedJob.State,
	)

	listWidth := m.width / 3
	detailsWidth := m.width - listWidth

	listStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true).
		Width(listWidth - 2)
	detailsStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true).
		Width(detailsWidth - 2).
		PaddingLeft(1).
		PaddingRight(1)

	view := lipgloss.JoinHorizontal(lipgloss.Top,
		listStyle.Render(listView),
		detailsStyle.Render(details),
	)

	return docStyle.Render(view)
}

func main() {
	jobs := []list.Item{
		slurm.JobInfo{
			JobID:   "123456",
			JobName: "GIGO",
			User:    "test",
			Account: "test",
			State:   slurm.Running,
		},
		slurm.JobInfo{
			JobID:   "75",
			JobName: "nerfstudio",
			User:    "test",
			Account: "test",
			State:   slurm.Failed,
		},
	}

	m := model{
		jobs: list.New(jobs, list.NewDefaultDelegate(), 0, 0),
	}
	m.jobs.Title = "Slurm Jobs"

	p := tea.NewProgram(m)
	_, err := p.Run()
	if err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

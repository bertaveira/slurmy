package main

import (
	"fmt"
	"os"
	"slurmy/slurm"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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
		listStyle := listStyle.Width(listWidth - 2) // Match the width used in View
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

	// Style for labels in the details view
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(highlight)

	// Build details string line by line
	jobIdLine := labelStyle.Render("Job ID:") + " " + selectedJob.JobID
	nameLine := labelStyle.Render("Name:") + " " + selectedJob.JobName
	// State is already handled in the list title, but we can show it here too if desired
	// stateLine := labelStyle.Render("Status:") + " " + selectedJob.State.String()
	userLine := labelStyle.Render("User:") + " " + selectedJob.User
	accountLine := labelStyle.Render("Account:") + " " + selectedJob.Account
	startTimeLine := labelStyle.Render("Start Time:") + " " + selectedJob.StartTime
	elapsedTimeLine := labelStyle.Render("Elapsed:") + " " + selectedJob.ElapsedTime
	allocCpusLine := labelStyle.Render("Alloc CPUs:") + " " + selectedJob.AllocCPUS
	allocTresLine := labelStyle.Render("Alloc TRES:") + " " + selectedJob.AllocTRES

	// Use JoinVertical for better control over details layout
	detailsContent := lipgloss.JoinVertical(lipgloss.Left,
		jobIdLine,
		nameLine,
		// stateLine, // Uncomment if you want state here too
		userLine,
		accountLine,
		startTimeLine,
		elapsedTimeLine,
		allocCpusLine,
		allocTresLine,
	)

	listWidth := m.width / 3
	detailsWidth := m.width - listWidth
	view := lipgloss.JoinHorizontal(lipgloss.Top,
		listStyle.Width(listWidth-2).Render(listView),
		listStyle.Width(detailsWidth-2).Padding(0, 1).Render(detailsContent),
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
		slurm.JobInfo{
			JobID:   "76",
			JobName: "nerfstudio",
			User:    "test",
			Account: "test",
			State:   slurm.Pending,
		},
		slurm.JobInfo{
			JobID:   "77",
			JobName: "nerfstudio",
			User:    "test",
			Account: "test",
			State:   slurm.Pending,
		},
		slurm.JobInfo{
			JobID:   "78",
			JobName: "nerfstudio",
			User:    "test",
			Account: "test",
			State:   slurm.Completed,
		},
		slurm.JobInfo{
			JobID:   "79",
			JobName: "nerfstudio",
			User:    "test",
			Account: "test",
			State:   slurm.Unknown,
		},
	}

	// Create a delegate and customize its styles
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(highlight).BorderLeftForeground(highlight)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(subtle).BorderLeftForeground(highlight) // Keep description subtle

	m := model{
		jobs: list.New(jobs, delegate, 0, 0), // Use the customized delegate
	}
	m.jobs.Title = "Slurm Jobs"

	// Style the list title
	m.jobs.Styles.Title = titleStyle

	p := tea.NewProgram(m)
	_, err := p.Run()
	if err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	"os"
	"os/user"
	"slurmy/slurm"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Add a message type for the timer tick
type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type model struct {
	jobs   list.Model
	width  int
	height int
	user   string
}

func (m model) Init() tea.Cmd {
	// Start the timer on initialization
	return tickCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd // Slice to hold multiple commands

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

	// Handle the tick message
	case tickMsg:
		// Fetch new job data
		sacctData, err := slurm.RunSacct(m.user)
		if err != nil {
			// Handle error fetching data (e.g., log it, show an error message)
			// For now, we just continue and wait for the next tick
			fmt.Fprintf(os.Stderr, "Error fetching sacct data: %v\n", err) // Log to stderr
			// Restart the timer even if fetching failed
			cmds = append(cmds, tickCmd())
		} else {
			// Convert slurm.JobInfo to []list.Item
			items := make([]list.Item, len(sacctData.Jobs))
			for i, job := range sacctData.Jobs {
				items[i] = job // slurm.JobInfo already implements list.Item
			}
			// Update the list items and restart the timer
			cmds = append(cmds, m.jobs.SetItems(items))
			cmds = append(cmds, tickCmd())
		}
	}

	// Update the list model and capture its command
	var listCmd tea.Cmd
	m.jobs, listCmd = m.jobs.Update(msg)
	cmds = append(cmds, listCmd) // Add the list's command

	// Return the updated model and combined commands
	return m, tea.Batch(cmds...)
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
	// Get linux user first
	currentUser, err := user.Current()
	if err != nil {
		fmt.Println("Error getting current user:", err)
		os.Exit(1)
	}
	username := currentUser.Username

	// Fetch initial job data
	initialSacctData, err := slurm.RunSacct(username)
	if err != nil {
		fmt.Printf("Error fetching initial sacct data: %v\n", err)
		// Decide how to handle initial error, maybe exit or start with empty list
		// os.Exit(1) // Option: Exit if initial fetch fails
	}

	// Prepare initial list items
	initialJobs := []list.Item{} // Start with empty list if fetch failed or no jobs
	if initialSacctData != nil {
		initialJobs = make([]list.Item, len(initialSacctData.Jobs))
		for i, job := range initialSacctData.Jobs {
			initialJobs[i] = job
		}
	}

	// Create a delegate and customize its styles
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(highlight).BorderLeftForeground(highlight)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(subtle).BorderLeftForeground(highlight) // Keep description subtle

	m := model{
		jobs: list.New(initialJobs, delegate, 0, 0), // Use fetched jobs
		user: username,                              // Store username in model
	}
	m.jobs.Title = "Your Slurm Jobs (last month)"

	// Style the list title
	m.jobs.Styles.Title = titleStyle

	p := tea.NewProgram(m, tea.WithAltScreen()) // Use AltScreen for better TUI experience
	_, err = p.Run()
	if err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

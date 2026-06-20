package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/user"
	"regexp"
	"slurmy/slurm"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type stdoutUpdateMsg struct {
	content  string
	filepath string // Track which file this update came from
}

type cancelResultMsg struct {
	jobID string
	err   error
}

type clearStatusMsg struct{}

type nodesMsg struct {
	nodes   []slurm.NodeInfo
	summary slurm.ClusterSummary
	err     error
}

type usageMsg struct {
	usages []slurm.UserUsage
	err    error
}

func fetchNodesCmd(c *slurm.Client) tea.Cmd {
	return func() tea.Msg {
		nodes, summary, err := c.GetNodes()
		return nodesMsg{nodes: nodes, summary: summary, err: err}
	}
}

func fetchUsageCmd(c *slurm.Client) tea.Cmd {
	return func() tea.Msg {
		usages, err := c.GetUserUsage()
		return usageMsg{usages: usages, err: err}
	}
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(text string) string {
	return ansiRegex.ReplaceAllString(text, "")
}

// cleanLine strips ANSI codes, resolves carriage returns to their final visible
// state (mimicking terminal overwrite behaviour), and removes other control chars.
func cleanLine(line string) string {
	line = stripANSI(line)
	if strings.Contains(line, "\r") {
		parts := strings.Split(line, "\r")
		line = parts[len(parts)-1]
	}
	line = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || (r >= 32 && r != 127) {
			return r
		}
		return -1
	}, line)
	return line
}

// readLastLines reads the last maxLines lines from file by seeking backwards in
// 64 KB chunks — the same strategy used by `tail`. This avoids scanning the
// whole file and has no per-line size limit.
func readLastLines(file *os.File, maxLines, maxLineLen int) (lines []string, skipped int, err error) {
	info, err := file.Stat()
	if err != nil {
		return nil, 0, err
	}
	if info.Size() == 0 {
		return nil, 0, nil
	}

	const chunkSize = 64 * 1024
	buf := make([]byte, chunkSize)
	remaining := info.Size()
	partial := ""

	processLine := func(raw string) {
		if strings.Contains(raw, "\r") {
			parts := strings.Split(raw, "\r")
			raw = parts[len(parts)-1]
		}
		raw = cleanLine(raw)
		if len(raw) > maxLineLen*10 {
			skipped++
			return
		}
		if len(raw) > maxLineLen {
			raw = raw[:maxLineLen] + "... [truncated]"
		}
		if strings.TrimSpace(raw) != "" {
			lines = append([]string{raw}, lines...)
		}
	}

	for remaining > 0 && len(lines) < maxLines {
		readSize := int64(chunkSize)
		if remaining < readSize {
			readSize = remaining
		}
		pos := remaining - readSize
		if _, err = file.Seek(pos, io.SeekStart); err != nil {
			return
		}
		n, readErr := file.Read(buf[:readSize])
		if readErr != nil && readErr != io.EOF {
			err = readErr
			return
		}
		if n == 0 {
			break
		}

		chunk := string(buf[:n]) + partial
		chunkLines := strings.Split(chunk, "\n")
		partial = chunkLines[0]

		for i := len(chunkLines) - 1; i >= 1 && len(lines) < maxLines; i-- {
			processLine(chunkLines[i])
		}
		remaining -= int64(n)
	}

	// Handle the very first line of the file
	if remaining == 0 && partial != "" && len(lines) < maxLines {
		processLine(partial)
	}

	return
}

func readStdoutCmd(filepath string) tea.Cmd {
	return func() tea.Msg {
		if filepath == "" {
			return stdoutUpdateMsg{content: "No stdout file available", filepath: filepath}
		}

		file, err := os.Open(filepath)
		if err != nil {
			return stdoutUpdateMsg{content: fmt.Sprintf("Error opening file: %v", err), filepath: filepath}
		}
		defer file.Close()

		const maxLines = 1000
		const maxLineLen = 10000

		lines, skipped, err := readLastLines(file, maxLines, maxLineLen)
		if err != nil {
			// Fallback: forward scan with a large buffer
			file.Seek(0, io.SeekStart)
			scanner := bufio.NewScanner(file)
			scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
			lines = lines[:0]
			skipped = 0
			for scanner.Scan() {
				line := cleanLine(scanner.Text())
				if len(line) > maxLineLen*10 {
					skipped++
					continue
				}
				if len(line) > maxLineLen {
					line = line[:maxLineLen] + "... [truncated]"
				}
				if strings.TrimSpace(line) != "" {
					lines = append(lines, line)
					if len(lines) > maxLines {
						lines = lines[1:]
					}
				}
			}
			if scanErr := scanner.Err(); scanErr != nil {
				suffix := fmt.Sprintf("\n\n[Warning: %v]", scanErr)
				if strings.Contains(scanErr.Error(), "token too long") {
					suffix = "\n\n[Note: some lines exceeded 10 MB and were skipped]"
				}
				if len(lines) == 0 {
					return stdoutUpdateMsg{content: "Error reading file" + suffix, filepath: filepath}
				}
				return stdoutUpdateMsg{content: strings.Join(lines, "\n") + suffix, filepath: filepath}
			}
		}

		if len(lines) == 0 {
			return stdoutUpdateMsg{content: "No output yet", filepath: filepath}
		}

		content := strings.Join(lines, "\n")
		if skipped > 0 {
			content += fmt.Sprintf("\n\n[Note: %d line(s) skipped — exceeded size limit]", skipped)
		}
		return stdoutUpdateMsg{content: content, filepath: filepath}
	}
}

func tailStdoutCmd(filepath string) tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return readStdoutCmd(filepath)()
	})
}

type model struct {
	jobs         list.Model
	stdoutView   viewport.Model
	width        int
	height       int
	slurmClient  *slurm.Client
	currentJobID string
	currentFile  string // resolved path of the stream currently being tailed

	// Output pane state
	showStderr    bool // tail stderr instead of stdout
	outputFocused bool // output pane has focus for scrolling (auto-tail paused)

	// Tabs
	activeTab   tab
	clusterView viewport.Model
	usersView   viewport.Model

	// Cluster tab data
	nodes      []slurm.NodeInfo
	clusterSum slurm.ClusterSummary
	clusterErr error

	// Users tab data
	usages   []slurm.UserUsage
	usageErr error

	// Confirmation modal state
	showConfirm    bool
	confirmJobID   string
	confirmJobName string
	cancelStatus   string // shows success/error message briefly
}

// streamPath returns the resolved path of the active output stream (stdout or
// stderr) for the given job.
func (m model) streamPath(j slurm.JobInfo) string {
	if m.showStderr {
		return j.ResolveStdErr()
	}
	return j.ResolveStdOut()
}

// loadStreamCmd switches the output pane to the active stream of the selected
// job and returns the command that reads it. It resets the viewport to the top
// and clears focus-scroll so the new stream tails from the bottom.
func (m *model) loadStreamCmd() tea.Cmd {
	m.currentFile = ""
	if item, ok := m.jobs.SelectedItem().(slurm.JobInfo); ok {
		m.currentFile = m.streamPath(item)
	}
	m.stdoutView.SetContent("Loading...")
	m.stdoutView.GotoTop()
	return readStdoutCmd(m.currentFile)
}

// refreshTabCmd returns the data-fetch command for the active tab (if any).
func (m model) refreshTabCmd() tea.Cmd {
	switch m.activeTab {
	case tabCluster:
		return fetchNodesCmd(m.slurmClient)
	case tabUsers:
		return fetchUsageCmd(m.slurmClient)
	}
	return nil
}

func (m model) Init() tea.Cmd {
	var stdoutPath string
	if item, ok := m.jobs.SelectedItem().(slurm.JobInfo); ok {
		stdoutPath = item.ResolveStdOut()
		m.currentFile = stdoutPath
	}
	return tea.Batch(tickCmd(), readStdoutCmd(stdoutPath))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle confirmation modal keys first
		if m.showConfirm {
			switch msg.String() {
			case "y", "Y", "enter":
				m.showConfirm = false
				jobID := m.confirmJobID
				client := m.slurmClient
				return m, func() tea.Msg {
					err := client.CancelJob(jobID)
					return cancelResultMsg{jobID: jobID, err: err}
				}
			case "n", "N", "esc", "escape":
				m.showConfirm = false
				m.confirmJobID = ""
				m.confirmJobName = ""
				return m, nil
			}
			return m, nil
		}

		// Tab switching — ignore while the job list filter is being typed.
		if m.jobs.FilterState() != list.Filtering {
			switch msg.String() {
			case "tab":
				m.activeTab = (m.activeTab + 1) % 3
				return m, m.refreshTabCmd()
			case "shift+tab":
				m.activeTab = (m.activeTab + 2) % 3
				return m, m.refreshTabCmd()
			case "1":
				m.activeTab = tabJobs
				return m, nil
			case "2":
				m.activeTab = tabCluster
				return m, m.refreshTabCmd()
			case "3":
				m.activeTab = tabUsers
				return m, m.refreshTabCmd()
			}
		}

		// Output-pane keys (Jobs tab only, not while filtering the job list).
		if m.activeTab == tabJobs && m.jobs.FilterState() != list.Filtering {
			switch msg.String() {
			case "o":
				if m.showStderr {
					m.showStderr = false
					return m, m.loadStreamCmd()
				}
				return m, nil
			case "e":
				if !m.showStderr {
					m.showStderr = true
					return m, m.loadStreamCmd()
				}
				return m, nil
			case "enter":
				m.outputFocused = !m.outputFocused
				return m, nil
			case "esc", "escape":
				if m.outputFocused {
					m.outputFocused = false
					m.stdoutView.GotoBottom() // resume following the latest output
					return m, nil
				}
			}
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "c":
			if m.activeTab == tabJobs {
				if item, ok := m.jobs.SelectedItem().(slurm.JobInfo); ok {
					if item.State == slurm.Running || item.State == slurm.Pending {
						m.showConfirm = true
						m.confirmJobID = item.JobID
						m.confirmJobName = item.JobName
					}
				}
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		listWidth := m.width / 3
		if listWidth < 2 {
			listWidth = 2
		}

		h, v := docStyle.GetFrameSize()
		lStyle := listStyle.Width(listWidth - 2)
		m.jobs.SetSize(listWidth-h, m.height-v-tabBarHeight-lStyle.GetVerticalFrameSize())

		detailsWidth := m.width - listWidth
		availableHeight := m.height - v - tabBarHeight
		// details box: 11 content lines + 2 borders + 2 padding
		// stdout box:  1 title + 2 borders + 2 padding  (viewport fills the rest)
		detailsHeight := 11 + 2 + 2
		stdoutExtra := 1 + 2 + 2
		stdoutHeight := availableHeight - detailsHeight - stdoutExtra
		if max := int(float64(availableHeight) * 0.6); stdoutHeight > max {
			stdoutHeight = max
		}
		if stdoutHeight < 5 {
			stdoutHeight = 5
		}

		m.stdoutView.Width = detailsWidth - 4
		m.stdoutView.Height = stdoutHeight
		if m.stdoutView.View() == "" {
			m.stdoutView.SetContent("Loading...")
		}

		// Full-width viewports for the Cluster and Users tabs, below the tab bar.
		tabContentWidth := m.width - h
		if tabContentWidth < 10 {
			tabContentWidth = 10
		}
		tabContentHeight := availableHeight - 2 // tab bar + spacer
		if tabContentHeight < 3 {
			tabContentHeight = 3
		}
		m.clusterView.Width = tabContentWidth
		m.clusterView.Height = tabContentHeight
		m.usersView.Width = tabContentWidth
		m.usersView.Height = tabContentHeight
		m.clusterView.SetContent(renderClusterView(m.nodes, m.clusterSum, m.clusterErr, tabContentWidth))
		m.usersView.SetContent(renderUsersView(m.usages, m.usageErr, tabContentWidth))

	case nodesMsg:
		m.nodes = msg.nodes
		m.clusterSum = msg.summary
		m.clusterErr = msg.err
		m.clusterView.SetContent(renderClusterView(m.nodes, m.clusterSum, m.clusterErr, m.clusterView.Width))

	case usageMsg:
		m.usages = msg.usages
		m.usageErr = msg.err
		m.usersView.SetContent(renderUsersView(m.usages, m.usageErr, m.usersView.Width))

	case stdoutUpdateMsg:
		if msg.filepath == m.currentFile {
			// Follow the latest output unless the user has scrolled up in the
			// focused pane — then hold their scroll position.
			prevOffset := m.stdoutView.YOffset
			wasAtBottom := m.stdoutView.AtBottom()
			m.stdoutView.SetContent(msg.content)
			if !m.outputFocused || wasAtBottom {
				m.stdoutView.GotoBottom()
			} else {
				m.stdoutView.SetYOffset(prevOffset)
			}
			if m.currentFile != "" {
				cmds = append(cmds, tailStdoutCmd(m.currentFile))
			}
		}

	case tickMsg:
		jobs, err := m.slurmClient.GetJobs()
		if err != nil {
			fmt.Fprintf(os.Stderr, "sacct error: %v\n", err)
			cmds = append(cmds, tickCmd())
		} else {
			items := make([]list.Item, len(jobs))
			for i, job := range jobs {
				items[i] = job
			}
			cmds = append(cmds, m.jobs.SetItems(items), tickCmd())
		}
		// Keep the active tab's data fresh too.
		if c := m.refreshTabCmd(); c != nil {
			cmds = append(cmds, c)
		}

	case cancelResultMsg:
		if msg.err != nil {
			m.cancelStatus = fmt.Sprintf("Failed to cancel job %s: %v", msg.jobID, msg.err)
		} else {
			m.cancelStatus = fmt.Sprintf("Job %s cancelled", msg.jobID)
		}
		m.confirmJobID = ""
		m.confirmJobName = ""
		cmds = append(cmds, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return clearStatusMsg{}
		}))
	
	case clearStatusMsg:
		m.cancelStatus = ""
	}

	// Keyboard input is routed only to the active tab's widgets; all other
	// messages (resize, ticks) propagate to every widget so they stay current.
	_, isKey := msg.(tea.KeyMsg)

	if !isKey || m.activeTab == tabJobs {
		// When the output pane is focused, keystrokes scroll it instead of
		// moving the job-list selection.
		if isKey && m.outputFocused {
			var vpCmd tea.Cmd
			m.stdoutView, vpCmd = m.stdoutView.Update(msg)
			cmds = append(cmds, vpCmd)
		} else {
			var listCmd tea.Cmd
			m.jobs, listCmd = m.jobs.Update(msg)
			cmds = append(cmds, listCmd)

			if item, ok := m.jobs.SelectedItem().(slurm.JobInfo); ok {
				if item.JobID != m.currentJobID {
					m.currentJobID = item.JobID
					m.currentFile = m.streamPath(item)
					m.stdoutView.SetContent("Loading...")
					m.stdoutView.GotoTop()
					cmds = append(cmds, readStdoutCmd(m.currentFile))
				}
			}

			// Non-key messages (resize, etc.) still reach the viewport.
			if !isKey {
				var vpCmd tea.Cmd
				m.stdoutView, vpCmd = m.stdoutView.Update(msg)
				cmds = append(cmds, vpCmd)
			}
		}
	}

	if !isKey || m.activeTab == tabCluster {
		var cvCmd tea.Cmd
		m.clusterView, cvCmd = m.clusterView.Update(msg)
		cmds = append(cmds, cvCmd)
	}

	if !isKey || m.activeTab == tabUsers {
		var uvCmd tea.Cmd
		m.usersView, uvCmd = m.usersView.Update(msg)
		cmds = append(cmds, uvCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	var selectedJob slurm.JobInfo
	if item, ok := m.jobs.SelectedItem().(slurm.JobInfo); ok {
		selectedJob = item
	}

	label := lipgloss.NewStyle().Bold(true).Foreground(highlight)
	faint := lipgloss.NewStyle().Faint(true)

	// Always show both output paths; the inactive stream is dimmed and the
	// active (tailed) one keeps the bright label.
	stdoutPath := selectedJob.ResolveStdOut()
	if stdoutPath == "" {
		stdoutPath = "—"
	}
	stderrPath := selectedJob.ResolveStdErr()
	if stderrPath == "" {
		stderrPath = "(merged into stdout / not set)"
	}
	outRow := label.Render("StdOut:") + " " + stdoutPath
	errRow := label.Render("StdErr:") + " " + stderrPath
	if m.showStderr {
		outRow = faint.Render("StdOut: " + stdoutPath)
	} else {
		errRow = faint.Render("StdErr: " + stderrPath)
	}
	if selectedJob.State == slurm.Pending {
		// Pending jobs come from squeue, which has no output paths yet.
		outRow = label.Render("Reason:") + " " + selectedJob.Reason
		errRow = ""
	}
	node := selectedJob.NodeList
	if node == "" {
		node = "—"
	}
	details := lipgloss.JoinVertical(lipgloss.Left,
		label.Render("Job ID:")+" "+selectedJob.JobID,
		label.Render("Name:")+" "+selectedJob.JobName,
		label.Render("User:")+" "+selectedJob.User,
		label.Render("Account:")+" "+selectedJob.Account,
		label.Render("Start Time:")+" "+selectedJob.StartTime,
		label.Render("Elapsed:")+" "+selectedJob.ElapsedTime,
		label.Render("Alloc CPUs:")+" "+selectedJob.AllocCPUS,
		label.Render("Alloc TRES:")+" "+selectedJob.AllocTRES,
		label.Render("Node:")+" "+node,
		outRow,
		errRow,
	)

	listWidth := m.width / 3
	detailsWidth := m.width - listWidth

	_, v := docStyle.GetFrameSize()
	availableHeight := m.height - v - tabBarHeight
	detailsHeight := 11 + 2 + 2
	stdoutExtra := 1 + 2 + 2
	stdoutHeight := availableHeight - detailsHeight - stdoutExtra
	if max := int(float64(availableHeight) * 0.6); stdoutHeight > max {
		stdoutHeight = max
	}
	if stdoutHeight < 5 {
		stdoutHeight = 5
	}

	stdoutContent := m.stdoutView.View()
	if stdoutContent == "" {
		stdoutContent = "No output yet"
	}

	outStyle := listStyle
	if m.outputFocused {
		outStyle = outStyle.BorderForeground(highlight)
	}
	rightColumn := lipgloss.JoinVertical(lipgloss.Top,
		listStyle.Width(detailsWidth-2).Padding(0, 1).Render(details),
		lipgloss.JoinVertical(lipgloss.Top,
			renderOutputHeader(m.showStderr, m.outputFocused),
			outStyle.Width(detailsWidth-2).Height(stdoutHeight).Padding(0, 1).Render(stdoutContent),
		),
	)

	var body string
	switch m.activeTab {
	case tabCluster:
		body = m.clusterView.View()
	case tabUsers:
		body = m.usersView.View()
	default:
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			listStyle.Width(listWidth-2).Render(m.jobs.View()),
			rightColumn,
		)
	}

	tabBar := renderTabBar(m.activeTab, m.width)
	mainView := docStyle.Render(lipgloss.JoinVertical(lipgloss.Left, tabBar, "", body))

	// Show status message at the bottom if present
	if m.cancelStatus != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#333333")).
			Padding(0, 2)
		mainView = lipgloss.JoinVertical(lipgloss.Left, mainView, statusStyle.Render(m.cancelStatus))
	}

	// Show confirmation modal overlay
	if m.showConfirm {
		modalStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#F5A623")).
			Padding(1, 2).
			Width(50).
			Align(lipgloss.Center)

		modalContent := lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F5A623")).Render("Cancel Job?"),
			"",
			fmt.Sprintf("Job ID: %s", m.confirmJobID),
			fmt.Sprintf("Name: %s", m.confirmJobName),
			"",
			lipgloss.NewStyle().Faint(true).Render("[y] Yes  [n] No"),
		)

		modal := modalStyle.Render(modalContent)

		// Center the modal on screen
		modalWidth := lipgloss.Width(modal)
		modalHeight := lipgloss.Height(modal)
		x := (m.width - modalWidth) / 2
		y := (m.height - modalHeight) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}

		modal = lipgloss.NewStyle().
			MarginLeft(x).
			MarginTop(y).
			Render(modal)

		mainView = lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, modal)
	}

	return mainView
}

func main() {
	currentUser, err := user.Current()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting current user:", err)
		os.Exit(1)
	}

	slurmClient, err := slurm.NewClient(currentUser.Username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing SLURM client: %v\n", err)
		os.Exit(1)
	}

	initialJobInfos, err := slurmClient.GetJobs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: initial job fetch failed: %v\n", err)
	}

	initialJobs := make([]list.Item, len(initialJobInfos))
	for i, job := range initialJobInfos {
		initialJobs[i] = job
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(highlight).BorderLeftForeground(highlight)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(subtle).BorderLeftForeground(highlight)

	vp := viewport.New(0, 0)
	vp.SetContent("Loading...")

	clusterVP := viewport.New(0, 0)
	clusterVP.SetContent("Loading cluster status…")
	usersVP := viewport.New(0, 0)
	usersVP.SetContent("Loading user usage…")

	m := model{
		jobs:        list.New(initialJobs, delegate, 0, 0),
		slurmClient: slurmClient,
		stdoutView:  vp,
		clusterView: clusterVP,
		usersView:   usersVP,
	}
	m.jobs.Title = "Your Slurm Jobs (last 30 days)"
	m.jobs.Styles.Title = titleStyle

	if len(initialJobs) > 0 {
		if item, ok := initialJobs[0].(slurm.JobInfo); ok {
			m.currentJobID = item.JobID
			m.currentFile = item.ResolveStdOut()
		}
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

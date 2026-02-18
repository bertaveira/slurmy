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
	content string
	filepath string // Track which file this update came from
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
	jobs          list.Model
	stdoutView    viewport.Model
	width         int
	height        int
	user          string
	currentJobID  string
	currentStdOut string
}

func (m model) Init() tea.Cmd {
	var stdoutPath string
	if item, ok := m.jobs.SelectedItem().(slurm.JobInfo); ok {
		stdoutPath = item.ResolveStdOut()
		m.currentStdOut = stdoutPath
	}
	return tea.Batch(tickCmd(), readStdoutCmd(stdoutPath))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
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
		m.jobs.SetSize(listWidth-h, m.height-v-lStyle.GetVerticalFrameSize())

		detailsWidth := m.width - listWidth
		availableHeight := m.height - v
		// details box: 10 content lines + 2 borders + 2 padding
		// stdout box:  1 title + 2 borders + 2 padding  (viewport fills the rest)
		detailsHeight := 10 + 2 + 2
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

	case stdoutUpdateMsg:
		if msg.filepath == m.currentStdOut {
			m.stdoutView.SetContent(msg.content)
			m.stdoutView.GotoBottom()
			if m.currentStdOut != "" {
				cmds = append(cmds, tailStdoutCmd(m.currentStdOut))
			}
		}

	case tickMsg:
		sacctData, err := slurm.RunSacct(m.user)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sacct error: %v\n", err)
			cmds = append(cmds, tickCmd())
		} else {
			squeueJobs, _ := slurm.RunSqueue(m.user)
			items := mergeJobs(squeueJobs, sacctData.Jobs)
			cmds = append(cmds, m.jobs.SetItems(items), tickCmd())
		}
	}

	var listCmd tea.Cmd
	m.jobs, listCmd = m.jobs.Update(msg)
	cmds = append(cmds, listCmd)

	if item, ok := m.jobs.SelectedItem().(slurm.JobInfo); ok {
		if item.JobID != m.currentJobID {
			m.currentJobID = item.JobID
			m.currentStdOut = item.ResolveStdOut()
			m.stdoutView.SetContent("Loading...")
			m.stdoutView.GotoTop()
			cmds = append(cmds, readStdoutCmd(m.currentStdOut))
		}
	}

	var vpCmd tea.Cmd
	m.stdoutView, vpCmd = m.stdoutView.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	var selectedJob slurm.JobInfo
	if item, ok := m.jobs.SelectedItem().(slurm.JobInfo); ok {
		selectedJob = item
	}

	label := lipgloss.NewStyle().Bold(true).Foreground(highlight)
	lastDetailRow := label.Render("StdOut:") + " " + selectedJob.ResolveStdOut()
	if selectedJob.State == slurm.Pending {
		lastDetailRow = label.Render("Reason:") + " " + selectedJob.Reason
	}
	node := selectedJob.NodeList
	if node == "" {
		node = "—"
	}
	details := lipgloss.JoinVertical(lipgloss.Left,
		label.Render("Job ID:")     +" "+selectedJob.JobID,
		label.Render("Name:")       +" "+selectedJob.JobName,
		label.Render("User:")       +" "+selectedJob.User,
		label.Render("Account:")    +" "+selectedJob.Account,
		label.Render("Start Time:") +" "+selectedJob.StartTime,
		label.Render("Elapsed:")    +" "+selectedJob.ElapsedTime,
		label.Render("Alloc CPUs:") +" "+selectedJob.AllocCPUS,
		label.Render("Alloc TRES:") +" "+selectedJob.AllocTRES,
		label.Render("Node:")       +" "+node,
		lastDetailRow,
	)

	listWidth := m.width / 3
	detailsWidth := m.width - listWidth

	_, v := docStyle.GetFrameSize()
	availableHeight := m.height - v
	detailsHeight := 10 + 2 + 2
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

	rightColumn := lipgloss.JoinVertical(lipgloss.Top,
		listStyle.Width(detailsWidth-2).Padding(0, 1).Render(details),
		lipgloss.JoinVertical(lipgloss.Top,
			titleStyle.Render("StdOut (tail -f)"),
			listStyle.Width(detailsWidth-2).Height(stdoutHeight).Padding(0, 1).Render(stdoutContent),
		),
	)

	return docStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top,
		listStyle.Width(listWidth-2).Render(m.jobs.View()),
		rightColumn,
	))
}

// mergeJobs combines pending jobs from squeue with historical jobs from sacct.
// squeue jobs come first (pending at the top); duplicates are deduplicated by JobID.
func mergeJobs(squeueJobs []slurm.JobInfo, sacctJobs []slurm.JobInfo) []list.Item {
	seen := make(map[string]bool, len(sacctJobs))
	items := make([]list.Item, 0, len(squeueJobs)+len(sacctJobs))

	for _, job := range squeueJobs {
		if !seen[job.JobID] {
			seen[job.JobID] = true
			items = append(items, job)
		}
	}
	for _, job := range sacctJobs {
		if !seen[job.JobID] {
			seen[job.JobID] = true
			items = append(items, job)
		}
	}
	return items
}

func main() {
	currentUser, err := user.Current()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting current user:", err)
		os.Exit(1)
	}

	initialSacctData, err := slurm.RunSacct(currentUser.Username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: initial sacct fetch failed: %v\n", err)
	}

	initialSqueueJobs, _ := slurm.RunSqueue(currentUser.Username)

	var initialJobs []list.Item
	if initialSacctData != nil {
		initialJobs = mergeJobs(initialSqueueJobs, initialSacctData.Jobs)
	} else if len(initialSqueueJobs) > 0 {
		initialJobs = mergeJobs(initialSqueueJobs, nil)
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(highlight).BorderLeftForeground(highlight)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(subtle).BorderLeftForeground(highlight)

	vp := viewport.New(0, 0)
	vp.SetContent("Loading...")

	m := model{
		jobs:       list.New(initialJobs, delegate, 0, 0),
		user:       currentUser.Username,
		stdoutView: vp,
	}
	m.jobs.Title = "Your Slurm Jobs (last 30 days)"
	m.jobs.Styles.Title = titleStyle

	if len(initialJobs) > 0 {
		if item, ok := initialJobs[0].(slurm.JobInfo); ok {
			m.currentJobID = item.JobID
			m.currentStdOut = item.ResolveStdOut()
		}
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

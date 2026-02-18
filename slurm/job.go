package slurm

import (
	"fmt"
	"os/user"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type JobState int

const (
	Unknown JobState = iota
	Running
	Completed
	Failed
	Pending
	Canceled
)

func (s JobState) String() string {
	switch s {
	case Running:
		return "Running"
	case Completed:
		return "Completed"
	case Failed:
		return "Failed"
	case Pending:
		return "Pending"
	case Canceled:
		return "Canceled"
	default:
		return "Unknown"
	}
}

func stateFromString(s string) JobState {
	s = strings.ToLower(s)
	switch {
	case strings.Contains(s, "running"):
		return Running
	case strings.Contains(s, "completed"):
		return Completed
	case strings.Contains(s, "failed"):
		return Failed
	case strings.Contains(s, "pending"):
		return Pending
	case strings.Contains(s, "cancel"):
		return Canceled
	default:
		return Unknown
	}
}

var (
	stateBaseStyle = lipgloss.NewStyle().
			MarginLeft(1).
			MarginRight(1).
			Padding(0, 2).
			Italic(true).
			Foreground(lipgloss.Color("#EEEEEE"))
	colorRunning   = lipgloss.Color("#08F2CF")
	colorCompleted = lipgloss.Color("#08F298")
	colorFailed    = lipgloss.Color("#DB45BE")
	colorPending   = lipgloss.Color("#EEF572")
	colorUnknown   = lipgloss.Color("#CDAEB5")
	colorCanceled  = lipgloss.Color("#808080")
)

// ansiRegex matches ANSI escape sequences used to strip terminal control codes.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(text string) string {
	return ansiRegex.ReplaceAllString(text, "")
}

type JobInfo struct {
	JobID       string
	JobName     string
	User        string
	Account     string
	State       JobState
	StartTime   string
	ElapsedTime string
	TimeLimit   string
	AllocCPUS   string
	AllocTRES   string
	StdOut      string
}

// Title implements the bubbletea list.Item interface.
func (j JobInfo) Title() string {
	var stateStyle lipgloss.Style
	switch j.State {
	case Running:
		stateStyle = stateBaseStyle.Background(colorRunning).Foreground(lipgloss.Color("#1C1C1C"))
	case Completed:
		stateStyle = stateBaseStyle.Background(colorCompleted).Foreground(lipgloss.Color("#1C1C1C"))
	case Failed:
		stateStyle = stateBaseStyle.Background(colorFailed)
	case Pending:
		stateStyle = stateBaseStyle.Background(colorPending)
	case Canceled:
		stateStyle = stateBaseStyle.Background(colorCanceled)
	default:
		stateStyle = stateBaseStyle.Background(colorUnknown)
	}
	return fmt.Sprintf("%s %s / %s", stateStyle.Render(j.State.String()), j.JobID, j.JobName)
}

// Description implements the bubbletea list.Item interface.
func (j JobInfo) Description() string {
	return fmt.Sprintf("%s | %s | %s", j.User, j.Account, j.AllocTRES)
}

// FilterValue implements the bubbletea list.Item interface.
func (j JobInfo) FilterValue() string {
	return j.JobID
}

// ResolveStdOut resolves SLURM filename pattern variables in the stdout path:
//   - %u  username
//   - %A  job array ID (or job ID for non-array jobs)
//   - %a  job array index (empty for non-array jobs)
//   - %j  job ID
//   - %J  job ID with array index (e.g. "12345_1")
func (j JobInfo) ResolveStdOut() string {
	if j.StdOut == "" {
		return ""
	}

	path := j.StdOut

	username := j.User
	if u, err := user.Current(); err == nil {
		username = u.Username
	}

	// Parse array job ID and index from "12345" or "12345_1"
	jobID := j.JobID
	arrayID := jobID
	arrayIndex := ""
	if idx := strings.Index(jobID, "_"); idx != -1 {
		arrayID = jobID[:idx]
		arrayIndex = jobID[idx+1:]
	}

	path = strings.ReplaceAll(path, "%u", username)
	path = strings.ReplaceAll(path, "%A", arrayID)
	path = strings.ReplaceAll(path, "%a", arrayIndex)
	path = strings.ReplaceAll(path, "%j", arrayID)
	path = strings.ReplaceAll(path, "%J", jobID)

	return path
}

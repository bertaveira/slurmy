package slurm

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Define JobState type
type JobState int

// Define constants for job states using iota
const (
	Unknown JobState = iota
	Running
	Completed
	Failed
	Pending
	Canceled
	// Add other states as needed from sacct documentation
	// e.g., Cancelled, Timeout, NodeFail, Preempted, Suspended
)

// String method for JobState
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
	// Add cases for other states
	default:
		return "Unknown"
	}
}

// Helper function to convert string to JobState
func stateFromString(s string) JobState {
	s = strings.ToLower(s) // Convert to lowercase once for all checks
	
	if strings.Contains(s, "running") {
		return Running
	} else if strings.Contains(s, "completed") {
		return Completed
	} else if strings.Contains(s, "failed") {
		return Failed
	} else if strings.Contains(s, "pending") {
		return Pending
	} else if strings.Contains(s, "cancel") {
		return Canceled
	}
	// Add cases for other state strings from sacct
	
	// Optionally log or handle unknown states from sacct
	return Unknown
}

// Styles for job states (using Background)
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
	// StdOutFile  string
}

// Implement bubble tea List interface
func (j JobInfo) Title() string {
	var stateStyle lipgloss.Style // Use base style type
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
	// Render the state with its style
	stateStr := stateStyle.Render(j.State.String())
	// Prepend styled state to title
	return fmt.Sprintf("%s %s / %s", stateStr, j.JobID, j.JobName)
}

func (j JobInfo) Description() string {
	// Remove state from description
	return fmt.Sprintf("%s | %s | %s", j.User, j.Account, j.AllocTRES)
}

func (j JobInfo) FilterValue() string {
	return j.JobID
}

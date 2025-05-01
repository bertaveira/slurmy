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
	// Add cases for other states
	default:
		return "Unknown"
	}
}

// Helper function to convert string to JobState
func stateFromString(s string) JobState {
	switch strings.ToLower(s) { // Use ToLower for case-insensitivity
	case "running", "Running":
		return Running
	case "completed", "Completed":
		return Completed
	case "failed", "Failed":
		return Failed
	case "pending", "Pending":
		return Pending
	// Add cases for other state strings from sacct
	default:
		// Optionally log or handle unknown states from sacct
		fmt.Printf("Warning: Unknown job state string encountered: %s\n", s)
		return Unknown
	}
}

// Define state colors based on the palette
var (
	colorRunning   = lipgloss.Color("208") // Orange
	colorCompleted = lipgloss.Color("142") // Muted Green
	colorFailed    = lipgloss.Color("167") // Muted Red
	colorPending   = lipgloss.Color("244") // Medium Grey
	colorUnknown   = lipgloss.Color("238") // Darker Grey
)

// Styles for job states (using Background)
var (
	stateStyleRunning   = lipgloss.NewStyle().Background(colorRunning)
	stateStyleCompleted = lipgloss.NewStyle().Background(colorCompleted)
	stateStyleFailed    = lipgloss.NewStyle().Background(colorFailed)
	stateStylePending   = lipgloss.NewStyle().Background(colorPending)
	stateStyleUnknown   = lipgloss.NewStyle().Background(colorUnknown)
)

type JobInfo struct {
	JobID       string
	JobName     string
	User        string
	Account     string
	State       JobState
	StartTime   string
	EndTime     string
	ElapsedTime string
	AllocCPUS   string
	AllocTRES   string
	StdOutFile  string
}

// Implement bubble tea List interface
func (j JobInfo) Title() string {
	var stateStyle lipgloss.Style // Use base style type
	switch j.State {
	case Running:
		stateStyle = stateStyleRunning
	case Completed:
		stateStyle = stateStyleCompleted
	case Failed:
		stateStyle = stateStyleFailed
	case Pending:
		stateStyle = stateStylePending
	default:
		stateStyle = stateStyleUnknown
	}
	// Render the state with its style
	stateStr := stateStyle.Render(j.State.String())
	// Prepend styled state to title
	return fmt.Sprintf("%s | %s | %s", stateStr, j.JobID, j.JobName)
}

func (j JobInfo) Description() string {
	// Remove state from description
	return fmt.Sprintf("%s | %s | %s", j.User, j.Account, j.AllocTRES)
}

func (j JobInfo) FilterValue() string {
	return j.JobID
}

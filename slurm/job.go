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

// Styles for job states
var (
	stateStyleRunning   = lipgloss.NewStyle().Background(lipgloss.Color("208")) // Orange
	stateStyleCompleted = lipgloss.NewStyle().Background(lipgloss.Color("78"))  // Green
	stateStyleFailed    = lipgloss.NewStyle().Background(lipgloss.Color("196")) // Red
	stateStylePending   = lipgloss.NewStyle().Background(lipgloss.Color("242")) // Grey
	stateStyleUnknown   = lipgloss.NewStyle().Background(lipgloss.Color("236")) // Darker Grey
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
	var stateStr string
	switch j.State {
	case Running:
		stateStr = stateStyleRunning.Render(j.State.String())
	case Completed:
		stateStr = stateStyleCompleted.Render(j.State.String())
	case Failed:
		stateStr = stateStyleFailed.Render(j.State.String())
	case Pending:
		stateStr = stateStylePending.Render(j.State.String())
	default:
		stateStr = stateStyleUnknown.Render(j.State.String())
	}
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

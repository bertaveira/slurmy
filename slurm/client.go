package slurm

import (
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// desiredFields lists the sacct fields the app wants, in order of priority.
// Fields not available on this cluster will be skipped.
var desiredFields = []struct {
	Name  string
	Width int
}{
	{"JobID", 30},
	{"JobName", 50},
	{"User", 0},
	{"Account", 30},
	{"State", 0},
	{"Start", 0},
	{"Elapsed", 0},
	{"Timelimit", 0},
	{"AllocCPUS", 0},
	{"AllocTRES", 100},
	{"NodeList", 80},
	{"StdOut", 200},
}

// Client handles all SLURM interactions and maintains state like available fields.
type Client struct {
	Username        string
	AvailableFields map[string]bool
	formatString    string
}

// NewClient creates a new SLURM client for the given user.
// It detects available sacct fields on initialization.
func NewClient(username string) (*Client, error) {
	c := &Client{
		Username:        username,
		AvailableFields: make(map[string]bool),
	}

	if err := c.detectAvailableFields(); err != nil {
		return nil, fmt.Errorf("failed to detect sacct fields: %w", err)
	}

	c.buildFormatString()
	return c, nil
}

// detectAvailableFields runs `sacct --helpformat` to discover which fields
// are supported by this SLURM installation.
func (c *Client) detectAvailableFields() error {
	cmd := exec.Command("sacct", "--helpformat")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	for _, line := range strings.Split(string(output), "\n") {
		for _, field := range strings.Fields(line) {
			field = strings.TrimSpace(field)
			if field != "" {
				c.AvailableFields[field] = true
			}
		}
	}

	return nil
}

// buildFormatString creates the --format argument using only available fields.
func (c *Client) buildFormatString() {
	var parts []string
	for _, f := range desiredFields {
		if c.AvailableFields[f.Name] {
			if f.Width > 0 {
				parts = append(parts, fmt.Sprintf("%s%%-%d", f.Name, f.Width))
			} else {
				parts = append(parts, f.Name)
			}
		}
	}
	c.formatString = strings.Join(parts, ",")
}

// HasField returns true if the given field is available on this cluster.
func (c *Client) HasField(name string) bool {
	return c.AvailableFields[name]
}

// GetJobs fetches and merges jobs from both sacct (historical) and squeue (pending).
// Pending jobs from squeue appear first; duplicates are deduplicated by JobID.
func (c *Client) GetJobs() ([]JobInfo, error) {
	sacctJobs, sacctErr := c.runSacct()
	squeueJobs, _ := runSqueue(c.Username)

	if sacctErr != nil && len(squeueJobs) == 0 {
		return nil, sacctErr
	}

	return c.mergeJobs(squeueJobs, sacctJobs), nil
}

// runSacct fetches historical jobs using the dynamically built format string.
func (c *Client) runSacct() ([]JobInfo, error) {
	if c.formatString == "" {
		return nil, fmt.Errorf("no valid sacct fields available")
	}

	startDate := time.Now().AddDate(0, 0, -30).Format("2006-01-02")

	cmd := exec.Command("sacct",
		"--allocations",
		"--format="+c.formatString,
		"--user", c.Username,
		"--starttime", startDate,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	sacct := &Sacct{}
	if err := sacct.Parse(string(output)); err != nil {
		return nil, err
	}

	slices.Reverse(sacct.Jobs)
	return sacct.Jobs, nil
}

// mergeJobs combines pending jobs from squeue with historical jobs from sacct.
// squeue jobs come first (pending at the top); duplicates are deduplicated by JobID.
func (c *Client) mergeJobs(squeueJobs, sacctJobs []JobInfo) []JobInfo {
	seen := make(map[string]bool, len(sacctJobs))
	jobs := make([]JobInfo, 0, len(squeueJobs)+len(sacctJobs))

	for _, job := range squeueJobs {
		if !seen[job.JobID] {
			seen[job.JobID] = true
			jobs = append(jobs, job)
		}
	}
	for _, job := range sacctJobs {
		if !seen[job.JobID] {
			seen[job.JobID] = true
			jobs = append(jobs, job)
		}
	}
	return jobs
}

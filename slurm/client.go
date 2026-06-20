package slurm

import (
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"
)

// desiredFields lists the sacct fields the app wants, in order of priority.
// Fields not available on this cluster will be skipped.
var desiredFields = []string{
	"JobID",
	"JobName",
	"User",
	"Account",
	"State",
	"Start",
	"Elapsed",
	"Timelimit",
	"AllocCPUS",
	"AllocTRES",
	"NodeList",
	"StdOut",
	"StdErr",
}

// Client handles all SLURM interactions and maintains state like available fields.
type Client struct {
	Username        string
	AvailableFields map[string]bool
	formatString    string

	// Demo mode: when true, all SLURM calls return synthetic data instead of
	// shelling out, so the app can run anywhere for screenshots/recordings.
	Demo      bool
	mu        sync.Mutex
	demoJobs  []JobInfo
	demoNodes []NodeInfo
	demoSum   ClusterSummary
	demoUsage []UserUsage
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
	for _, name := range desiredFields {
		if c.AvailableFields[name] {
			parts = append(parts, name)
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
	if c.Demo {
		c.mu.Lock()
		defer c.mu.Unlock()
		return append([]JobInfo(nil), c.demoJobs...), nil
	}

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
		"--parsable2",
		"--format="+c.formatString,
		"--user", c.Username,
		"--starttime", startDate,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	sacct := &Sacct{}
	if err := sacct.ParseParsable(string(output)); err != nil {
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

// CancelJob cancels the job with the given ID using scancel.
func (c *Client) CancelJob(jobID string) error {
	if c.Demo {
		c.mu.Lock()
		defer c.mu.Unlock()
		for i := range c.demoJobs {
			if c.demoJobs[i].JobID == jobID {
				c.demoJobs[i].State = Canceled
				c.demoJobs[i].Reason = ""
			}
		}
		return nil
	}

	cmd := exec.Command("scancel", jobID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scancel failed: %w: %s", err, string(output))
	}
	return nil
}

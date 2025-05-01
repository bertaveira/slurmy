// Parse sacct output into a struct

package slurm

import (
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"
)

type Sacct struct {
	Jobs []JobInfo
}

func (s *Sacct) Parse(output string) error {
	lines := strings.Split(output, "\n")

	fieldNames := []string{}
	for _, line := range lines {
		if line == "" || strings.Contains(line, "-----") {
			continue
		}

		if strings.Contains(line, "JobID") {
			// Split line by spacing
			fieldNames = strings.Fields(line)
			continue
		}

		// Split by whitespace and filter out empty fields
		fields := []string{}
		for _, field := range strings.Fields(line) {
			if field != "" && field != " " {
				fields = append(fields, field)
			}
		}

		if strings.Contains(fields[0], ".ba") || strings.Contains(fields[0], ".ex") {
			continue
		}

		if len(fields) != len(fieldNames) {
			// Combine the format string into one line
			continue // Skip lines that don't match the header structure
		}

		job := parseJob(fields, fieldNames)
		s.Jobs = append(s.Jobs, job)
	}
	return nil
}

func parseJob(fields []string, fieldNames []string) JobInfo {
	job := JobInfo{}

	// Use a map for easier field assignment
	fieldMap := make(map[string]string)
	for i, name := range fieldNames {
		if i < len(fields) {
			fieldMap[name] = fields[i]
		}
	}

	var ok bool
	if job.JobID, ok = fieldMap["JobID"]; !ok { fmt.Println("Missing field: JobID") }
	if job.JobName, ok = fieldMap["JobName"]; !ok { fmt.Println("Missing field: JobName") }
	if job.User, ok = fieldMap["User"]; !ok { fmt.Println("Missing field: User") }
	if job.Account, ok = fieldMap["Account"]; !ok { fmt.Println("Missing field: Account") }
	if stateStr, ok := fieldMap["State"]; ok {
		job.State = stateFromString(stateStr)
	} else {
		fmt.Println("Missing field: State")
	}
	if job.StartTime, ok = fieldMap["Start"]; !ok { fmt.Println("Missing field: Start") }
	if job.ElapsedTime, ok = fieldMap["Elapsed"]; !ok { fmt.Println("Missing field: Elapsed") }
	if job.TimeLimit, ok = fieldMap["Timelimit"]; !ok { fmt.Println("Missing field: Timelimit") }
	if job.AllocCPUS, ok = fieldMap["AllocCPUS"]; !ok { fmt.Println("Missing field: AllocCPUS") }
	if job.AllocTRES, ok = fieldMap["AllocTRES"]; !ok { fmt.Println("Missing field: AllocTRES") }

	return job
}

func RunSacct(user string) (*Sacct, error) {
	// Calculate the date two weeks ago
	now := time.Now()
	twoWeeksAgo := now.AddDate(0, 0, -30)
	startDate := twoWeeksAgo.Format("2006-01-02") // Format date as YYYY-MM-DD

	// Add the --starttime flag to the sacct command
	cmd := exec.Command("sacct",
		"--allocations",
		"--format=JobID%-30,JobName%-50,User,Account%-30,State,Start,Elapsed,Timelimit,AllocCPUS,AllocTRES%-100",
		"--user", user,
		"--starttime", startDate, // Add start time filter
	)
	output, err := cmd.CombinedOutput() // Use CombinedOutput to capture stderr as well
	if err != nil {
		// Combine the format string into one line with explicit newlines
		fmt.Printf("Error running sacct: %v\nOutput:\n%s\n", err, string(output))
		return nil, fmt.Errorf("sacct command failed: %w", err)
	}
	sacct := &Sacct{}
	err = sacct.Parse(string(output))
	if err != nil {
		return nil, err
	}

	// Reverse the slice using the slices package
	slices.Reverse(sacct.Jobs)

	return sacct, nil
}

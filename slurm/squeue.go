package slurm

import (
	"os/exec"
	"strings"
)

// runSqueue fetches pending jobs for the given user via squeue and returns
// them as JobInfo values. Only PENDING jobs are returned — running jobs are
// already covered by sacct with richer data.
func runSqueue(username string) ([]JobInfo, error) {
	cmd := exec.Command("squeue",
		"--user", username,
		"--noheader",
		"-o", "%i|%j|%u|%a|%T|%S|%M|%l|%C|%R",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	var jobs []JobInfo
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "|", 10)
		if len(fields) < 10 {
			continue
		}

		state := stateFromString(cleanField(fields[4]))
		if state != Pending {
			continue
		}

		jobs = append(jobs, JobInfo{
			JobID:       cleanField(fields[0]),
			JobName:     cleanField(fields[1]),
			User:        cleanField(fields[2]),
			Account:     cleanField(fields[3]),
			State:       state,
			StartTime:   cleanField(fields[5]),
			ElapsedTime: cleanField(fields[6]),
			TimeLimit:   cleanField(fields[7]),
			AllocCPUS:   cleanField(fields[8]),
			Reason:      cleanField(fields[9]),
		})
	}

	return jobs, nil
}

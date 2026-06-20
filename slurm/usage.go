package slurm

import (
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// UserUsage aggregates a single user's current footprint on the cluster.
type UserUsage struct {
	User string

	RunningGPUs int
	RunningCPUs int
	RunningJobs int

	PendingGPUs int
	PendingJobs int
}

// GetUserUsage returns per-user resource usage across the whole cluster,
// aggregated from squeue. Running and pending jobs are tallied separately.
// The result is sorted by running GPUs (then pending GPUs) descending.
func (c *Client) GetUserUsage() ([]UserUsage, error) {
	cmd := exec.Command("squeue",
		"--noheader",
		"-O", "UserName:|,StateCompact:|,tres-alloc:|,NumCPUs:|",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	byUser := make(map[string]*UserUsage)
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "|")
		if len(fields) < 4 {
			continue
		}
		user := strings.TrimSpace(fields[0])
		state := strings.TrimSpace(fields[1])
		gpus := parseTresGPU(fields[2])
		cpus := atoiSafe(strings.TrimSpace(fields[3]))
		if user == "" {
			continue
		}

		u := byUser[user]
		if u == nil {
			u = &UserUsage{User: user}
			byUser[user] = u
		}

		switch state {
		case "R", "CG": // running / completing
			u.RunningJobs++
			u.RunningGPUs += gpus
			u.RunningCPUs += cpus
		case "PD": // pending
			u.PendingJobs++
			u.PendingGPUs += gpus
		default:
			// Treat other transient states as running for accounting purposes.
			u.RunningJobs++
			u.RunningGPUs += gpus
			u.RunningCPUs += cpus
		}
	}

	usages := make([]UserUsage, 0, len(byUser))
	for _, u := range byUser {
		usages = append(usages, *u)
	}
	sort.Slice(usages, func(i, j int) bool {
		if usages[i].RunningGPUs != usages[j].RunningGPUs {
			return usages[i].RunningGPUs > usages[j].RunningGPUs
		}
		if usages[i].PendingGPUs != usages[j].PendingGPUs {
			return usages[i].PendingGPUs > usages[j].PendingGPUs
		}
		return usages[i].RunningCPUs > usages[j].RunningCPUs
	})
	return usages, nil
}

// parseTresGPU extracts the gpu count from a TRES string such as
// "cpu=14,mem=252000M,node=1,billing=32,gres/gpu=1,gres/gpu:h200-141gb=1".
// The generic "gres/gpu=" key is preferred to avoid double-counting typed
// entries like "gres/gpu:h200-141gb=".
func parseTresGPU(s string) int {
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "gres/gpu=") {
			n, err := strconv.Atoi(part[len("gres/gpu="):])
			if err == nil {
				return n
			}
		}
	}
	return 0
}

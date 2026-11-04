package slurm

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// NodeJob is a live allocation visible on a node. CPUs and TRES describe the
// whole job, not an assumed equal share of a multi-node allocation.
type NodeJob struct {
	ID, Name, User, State, Partition string
	Elapsed, Limit, Remaining        string
	CPUs, TRES, Nodes                string
	NodeCount                        int
}

// GetNodeJobs queries all users' jobs on a node, independently of the Jobs tab's
// user filter. Array tasks are expanded so each allocation has its own row.
func (c *Client) GetNodeJobs(node string) ([]NodeJob, error) {
	if c.Demo {
		return c.demoNodeJobs(node), nil
	}
	cmd := exec.Command("squeue", "--all", "--array", "--nodes", node,
		"--noheader", "-O", "JobID:|,Name:|,UserName:|,State:|,Partition:|,TimeUsed:|,TimeLimit:|,TimeLeft:|,NumCPUs:|,tres-alloc:|,NumNodes:|,NodeList:|")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("squeue: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return parseNodeJobs(string(output)), nil
}

func parseNodeJobs(output string) []NodeJob {
	var jobs []NodeJob
	seen := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(strings.TrimSpace(line), "|")
		if len(fields) < 12 {
			continue
		}
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		if fields[0] == "" || seen[fields[0]] || fields[3] == "PENDING" {
			continue
		}
		seen[fields[0]] = true
		jobs = append(jobs, NodeJob{
			ID: fields[0], Name: fields[1], User: fields[2], State: fields[3], Partition: fields[4],
			Elapsed: fields[5], Limit: fields[6], Remaining: fields[7], CPUs: fields[8],
			TRES: fields[9], NodeCount: atoiSafe(fields[10]), Nodes: fields[11],
		})
	}
	return jobs
}

// TimePercent reports the fraction of the scheduler's wall-time limit consumed.
// Unlimited or unknown limits have no meaningful percentage.
func (j NodeJob) TimePercent() (int, bool) {
	elapsed, ok := slurmSeconds(j.Elapsed)
	limit, valid := slurmSeconds(j.Limit)
	if !ok || !valid || limit <= 0 {
		return 0, false
	}
	return min(100, int(elapsed*100/limit)), true
}

func slurmSeconds(value string) (int64, bool) {
	days := int64(0)
	if before, after, found := strings.Cut(value, "-"); found {
		n, err := strconv.ParseInt(before, 10, 64)
		if err != nil || n < 0 {
			return 0, false
		}
		days, value = n, after
	}
	parts := strings.Split(value, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	var seconds int64
	for _, part := range parts {
		n, err := strconv.ParseInt(part, 10, 64)
		if err != nil || n < 0 {
			return 0, false
		}
		seconds = seconds*60 + n
	}
	return days*86400 + seconds, true
}

func (c *Client) demoNodeJobs(node string) []NodeJob {
	c.mu.Lock()
	defer c.mu.Unlock()
	var jobs []NodeJob
	for _, job := range c.demoJobs {
		if job.State != Running || job.NodeList != node {
			continue
		}
		jobs = append(jobs, NodeJob{
			ID: job.JobID, Name: job.JobName, User: job.User, State: "RUNNING", Partition: "gpu",
			Elapsed: job.ElapsedTime, Limit: job.TimeLimit,
			CPUs: job.AllocCPUS, TRES: job.AllocTRES, Nodes: node, NodeCount: 1,
		})
	}
	for _, n := range c.demoNodes {
		if n.Name != node || n.CPUAlloc == 0 || !n.Available() {
			continue
		}
		cpus, gpus := n.CPUAlloc, n.GPUUsed
		for _, j := range jobs {
			cpus -= atoiSafe(j.CPUs)
			gpus -= parseTresGPU(j.TRES)
		}
		if cpus > 0 || gpus > 0 {
			jobs = append(jobs, NodeJob{
				ID: "184201", Name: "distributed_training", User: "alice", State: "RUNNING", Partition: n.Partition,
				Elapsed: "06:00:00", Limit: "1-00:00:00", Remaining: "18:00:00",
				CPUs: strconv.Itoa(max(0, cpus)), TRES: fmt.Sprintf("cpu=%d,mem=128G,gres/gpu=%d", max(0, cpus), max(0, gpus)),
				Nodes: node, NodeCount: 1,
			})
		}
	}
	// Demo limits are finite; derive remaining time from each job's actual limit.
	for i := range jobs {
		elapsed, _ := slurmSeconds(jobs[i].Elapsed)
		limit, _ := slurmSeconds(jobs[i].Limit)
		left := max(int64(0), limit-elapsed)
		jobs[i].Remaining = fmt.Sprintf("%d-%02d:%02d:%02d", left/86400, left/3600%24, left/60%60, left%60)
	}
	return jobs
}

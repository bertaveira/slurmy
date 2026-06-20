package slurm

import (
	"os/exec"
	"strconv"
	"strings"
)

// NodeInfo describes a single compute node and its current resource usage,
// parsed from sinfo.
type NodeInfo struct {
	Name      string
	State     string // long state, e.g. "mixed", "allocated", "idle", "drained"
	Partition string

	GPUTotal int
	GPUUsed  int
	GPUType  string // e.g. "H200-141GB" (empty for CPU-only nodes)

	CPUAlloc int
	CPUTotal int

	MemTotalMB int // total real memory in MB
	MemFreeMB  int // free memory in MB
}

// HasGPU reports whether the node advertises any GPUs.
func (n NodeInfo) HasGPU() bool { return n.GPUTotal > 0 }

// GPUFree returns the number of unallocated GPUs on this node. On unavailable
// nodes (down/drained), GPUs cannot be scheduled, so free is reported as 0.
func (n NodeInfo) GPUFree() int {
	if !n.Available() {
		return 0
	}
	free := n.GPUTotal - n.GPUUsed
	if free < 0 {
		return 0
	}
	return free
}

// Available reports whether the node can currently accept jobs. Nodes that are
// down, drained, in maintenance, or otherwise unreachable are not available.
func (n NodeInfo) Available() bool {
	s := strings.ToLower(strings.TrimRight(n.State, "*~#$+"))
	switch {
	case strings.Contains(s, "down"),
		strings.Contains(s, "drain"),
		strings.Contains(s, "drng"),
		strings.Contains(s, "maint"),
		strings.Contains(s, "fail"),
		strings.Contains(s, "unknown"),
		strings.Contains(s, "boot"),
		strings.Contains(s, "reboot"),
		strings.Contains(s, "power"),
		strings.Contains(s, "invalid"):
		return false
	}
	return true
}

// ClusterSummary aggregates resource usage across all nodes.
type ClusterSummary struct {
	NodesTotal     int
	NodesAvailable int
	NodesDown      int

	GPUTotal       int // all GPUs across all nodes
	GPUUsed        int // GPUs currently allocated
	GPUFree        int // GPUs free on available nodes
	GPUUnavailable int // GPUs on down/drained nodes

	CPUAlloc int
	CPUTotal int
}

// GetNodes fetches all cluster nodes via sinfo and returns them along with an
// aggregate summary. Every requested sinfo field is a single whitespace-free
// token, so each line splits cleanly into the expected columns.
func (c *Client) GetNodes() ([]NodeInfo, ClusterSummary, error) {
	cmd := exec.Command("sinfo",
		"--Node",
		"--noheader",
		"-O", "NodeList:|,StateLong:|,Gres:|,GresUsed:|,CPUsState:|,Partition:|,Memory:|,FreeMem:|",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, ClusterSummary{}, err
	}

	var nodes []NodeInfo
	var summary ClusterSummary

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, "|")
		// Expect 8 fields; trailing delimiter yields an empty 9th element.
		if len(fields) < 8 {
			continue
		}
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}

		gpuTotal, gpuType := parseGres(fields[2])
		gpuUsed, _ := parseGres(fields[3])
		cpuAlloc, cpuTotal := parseCPUsState(fields[4])

		node := NodeInfo{
			Name:       fields[0],
			State:      fields[1],
			GPUTotal:   gpuTotal,
			GPUUsed:    gpuUsed,
			GPUType:    gpuType,
			CPUAlloc:   cpuAlloc,
			CPUTotal:   cpuTotal,
			Partition:  fields[5],
			MemTotalMB: atoiSafe(fields[6]),
			MemFreeMB:  atoiSafe(fields[7]),
		}
		nodes = append(nodes, node)

		summary.NodesTotal++
		summary.GPUTotal += node.GPUTotal
		summary.CPUAlloc += node.CPUAlloc
		summary.CPUTotal += node.CPUTotal
		if node.Available() {
			summary.NodesAvailable++
			summary.GPUUsed += node.GPUUsed
			summary.GPUFree += node.GPUFree()
		} else {
			summary.NodesDown++
			summary.GPUUnavailable += node.GPUTotal
		}
	}

	return nodes, summary, nil
}

// parseGres extracts the GPU count and type from a Gres / GresUsed string such
// as "gpu:H200-141GB:8(S:0-1)" or "gpu:H200-141GB:7(IDX:0-2,4-7)". It returns
// (0, "") for "(null)" or non-GPU gres. Multiple comma-separated gpu entries
// are summed.
func parseGres(s string) (count int, gpuType string) {
	if s == "" || s == "(null)" || s == "N/A" {
		return 0, ""
	}
	for _, entry := range strings.Split(s, ",") {
		entry = strings.TrimSpace(entry)
		if !strings.HasPrefix(entry, "gpu:") {
			continue
		}
		// Drop any "(...)" suffix describing sockets or GPU indices.
		if i := strings.Index(entry, "("); i != -1 {
			entry = entry[:i]
		}
		parts := strings.Split(entry, ":")
		if len(parts) < 2 {
			continue
		}
		// Last part is the count; the middle part (if present) is the type.
		n, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			continue
		}
		count += n
		if gpuType == "" && len(parts) >= 3 {
			gpuType = parts[1]
		}
	}
	return count, gpuType
}

// parseCPUsState parses sinfo's "A/I/O/T" CPU state string (Allocated / Idle /
// Other / Total) and returns the allocated and total counts.
func parseCPUsState(s string) (alloc, total int) {
	parts := strings.Split(s, "/")
	if len(parts) != 4 {
		return 0, 0
	}
	return atoiSafe(parts[0]), atoiSafe(parts[3])
}

func atoiSafe(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

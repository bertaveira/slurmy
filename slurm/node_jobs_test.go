package slurm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetNodeJobs(t *testing.T) {
	dir := t.TempDir()
	// Verify the real command path selects a node, expands arrays, and does
	// not inherit the current user's Jobs-tab filter.
	script := `#!/bin/sh
test "$1" = "--all" && test "$2" = "--array" && test "$3" = "--nodes" && test "$4" = "gpu01" || exit 1
printf '%s\n' '99|training|other-user|RUNNING|gpu|01:00:00|04:00:00|03:00:00|16|cpu=16,gres/gpu=1|1|gpu01|'
`
	if err := os.WriteFile(filepath.Join(dir, "squeue"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	c := Client{Username: "current-user"}
	jobs, err := c.GetNodeJobs("gpu01")
	if err != nil || len(jobs) != 1 {
		t.Fatalf("GetNodeJobs() = %+v, %v", jobs, err)
	}
	if jobs[0].User != "other-user" || jobs[0].Remaining != "03:00:00" {
		t.Errorf("unexpected allocation: %+v", jobs[0])
	}
}

func TestParseNodeJobs(t *testing.T) {
	output := `123_1 | training run | alice | RUNNING | gpu | 1-06:00:00 | 2-00:00:00 | 18:00:00 | 128 | cpu=128,mem=512G,gres/gpu=8 | 2 | gpu-[01-02] |
123_2 | evaluation | bob | COMPLETING | gpu | 10:00 | UNLIMITED | UNLIMITED | 16 | cpu=16,gres/gpu=1 | 1 | gpu-01 |
123_1 | training run | alice | RUNNING | normal | 1-06:00:00 | 2-00:00:00 | 18:00:00 | 128 | cpu=128 | 2 | gpu-[01-02] |
124 | pending | alice | PENDING | gpu | 0:00 | 1:00:00 | 1:00:00 | 16 | N/A | 1 | gpu-01 |
malformed
`
	jobs := parseNodeJobs(output)
	if len(jobs) != 2 {
		t.Fatalf("got %d jobs, want 2: %+v", len(jobs), jobs)
	}
	want := NodeJob{
		ID: "123_1", Name: "training run", User: "alice", State: "RUNNING", Partition: "gpu",
		Elapsed: "1-06:00:00", Limit: "2-00:00:00", Remaining: "18:00:00",
		CPUs: "128", TRES: "cpu=128,mem=512G,gres/gpu=8", NodeCount: 2, Nodes: "gpu-[01-02]",
	}
	if jobs[0] != want {
		t.Errorf("job = %+v, want %+v", jobs[0], want)
	}
	if jobs[1].User != "bob" || jobs[1].ID != "123_2" {
		t.Errorf("array tasks and other users must be preserved: %+v", jobs[1])
	}
}

func TestNodeJobTimePercent(t *testing.T) {
	for _, tt := range []struct {
		elapsed, limit string
		percent        int
		valid          bool
	}{
		{"1-06:00:00", "2-00:00:00", 62, true},
		{"30:00", "01:00:00", 50, true},
		{"00:00", "01:00:00", 0, true},
		{"02:00:00", "01:00:00", 100, true},
		{"01:00:00", "UNLIMITED", 0, false},
		{"INVALID", "01:00:00", 0, false},
		{"01:00:00", "00:00", 0, false},
	} {
		t.Run(tt.elapsed+"/"+tt.limit, func(t *testing.T) {
			job := NodeJob{Elapsed: tt.elapsed, Limit: tt.limit}
			if got, valid := job.TimePercent(); got != tt.percent || valid != tt.valid {
				t.Errorf("TimePercent() = %d, %v; want %d, %v", got, valid, tt.percent, tt.valid)
			}
		})
	}
}

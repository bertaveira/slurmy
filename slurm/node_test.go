package slurm

import (
	"reflect"
	"testing"
)

func TestNodeMemoryUsage(t *testing.T) {
	for _, tt := range []struct {
		free            string
		used, available int
		known           bool
	}{
		{"256000", 256000, 256000, true},
		{"0", 512000, 0, true},
		{"600000", 0, 512000, true},
		{"-1", 0, 0, false},
		{"N/A", 0, 0, false},
		{"", 0, 0, false},
	} {
		t.Run(tt.free, func(t *testing.T) {
			nodes := parseNodes("node01|idle|(null)|(null)|0/64/0/64|normal|512000|" + tt.free + "|")
			used, free, known := nodes[0].MemoryUsage()
			if used != tt.used || free != tt.available || known != tt.known {
				t.Errorf("MemoryUsage() = %d, %d, %v; want %d, %d, %v", used, free, known, tt.used, tt.available, tt.known)
			}
		})
	}
}

func TestParseNodeReservedMemory(t *testing.T) {
	for _, value := range []struct {
		field string
		want  int
	}{{"4096", 4096}, {"0", 0}, {"N/A", -1}} {
		nodes := parseNodes("node01|mixed|(null)|(null)|1/63/0/64|normal|512000|256000|" + value.field + "|")
		if nodes[0].MemAllocMB != value.want {
			t.Errorf("AllocMem %q parsed as %d, want %d", value.field, nodes[0].MemAllocMB, value.want)
		}
	}
}

func TestParseNodesMultiplePartitions(t *testing.T) {
	output := `node01 | mixed | gpu:H200:8 | gpu:H200:2 | 16/48/0/64 | high | 512000 | 256000 |
node01 | mixed | gpu:H200:8 | gpu:H200:2 | 16/48/0/64 | normal | 512000 | 256000 |
node01 | mixed | gpu:H200:8 | gpu:H200:2 | 16/48/0/64 | low | 512000 | 256000 |
node01 | mixed | gpu:H200:8 | gpu:H200:2 | 16/48/0/64 | high | 512000 | 256000 |
`
	nodes := parseNodes(output)
	wantNodes := []NodeInfo{{
		Name: "node01", State: "mixed", Partition: "high,normal,low",
		GPUTotal: 8, GPUUsed: 2, GPUType: "H200",
		CPUAlloc: 16, CPUTotal: 64, MemTotalMB: 512000, MemFreeMB: 256000, MemAllocMB: -1,
	}}
	if !reflect.DeepEqual(nodes, wantNodes) {
		t.Fatalf("parseNodes() = %+v, want %+v", nodes, wantNodes)
	}
	wantSummary := ClusterSummary{
		NodesTotal: 1, NodesAvailable: 1,
		GPUTotal: 8, GPUUsed: 2, GPUFree: 6, CPUAlloc: 16, CPUTotal: 64,
	}
	if got := summarize(nodes); got != wantSummary {
		t.Errorf("summarize() = %+v, want %+v", got, wantSummary)
	}
}

func TestParseNodesInterleavedPartitions(t *testing.T) {
	output := `
node02 | drained | gpu:H200:4 | gpu:H200:0 | 0/0/32/32 | normal | 256000 | 128000 |
malformed row
node01 | mixed | gpu:H200:8 | gpu:H200:2 | 16/48/0/64 | high | 512000 | 256000 |
node02 | drained | gpu:H200:4 | gpu:H200:0 | 0/0/32/32 | low | 256000 | 128000 |
node03 | idle | (null) | (null) | 0/16/0/16 | normal | 64000 | 32000 |
node01 | mixed | gpu:H200:8 | gpu:H200:2 | 16/48/0/64 | normal | 512000 | 256000 |
`
	nodes := parseNodes(output)
	if len(nodes) != 3 {
		t.Fatalf("got %d nodes, want 3", len(nodes))
	}
	for i, want := range []struct{ name, partition string }{
		{"node02", "normal,low"},
		{"node01", "high,normal"},
		{"node03", "normal"},
	} {
		if nodes[i].Name != want.name || nodes[i].Partition != want.partition {
			t.Errorf("node %d = %+v, want name %q and partitions %q", i, nodes[i], want.name, want.partition)
		}
	}
	wantSummary := ClusterSummary{
		NodesTotal: 3, NodesAvailable: 2, NodesDown: 1,
		GPUTotal: 12, GPUUsed: 2, GPUFree: 6, GPUUnavailable: 4,
		CPUAlloc: 16, CPUTotal: 112,
	}
	if got := summarize(nodes); got != wantSummary {
		t.Errorf("summarize() = %+v, want %+v", got, wantSummary)
	}
}

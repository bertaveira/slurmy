package slurm

import (
	"reflect"
	"testing"
)

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
		CPUAlloc: 16, CPUTotal: 64, MemTotalMB: 512000, MemFreeMB: 256000,
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

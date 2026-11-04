package main

import (
	"slurmy/slurm"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

func TestClusterNodeNavigation(t *testing.T) {
	m := model{
		clusterView:  viewport.New(48, 12),
		nodes:        []slurm.NodeInfo{{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"}},
		selectedNode: "a",
	}
	for _, step := range []struct{ key, name string }{
		{"right", "b"}, {"down", "d"}, {"right", "d"}, {"up", "b"}, {"left", "a"}, {"left", "a"},
	} {
		m.handleClusterKey(step.key)
		if m.selectedNode != step.name {
			t.Fatalf("after %s selected %q, want %q", step.key, m.selectedNode, step.name)
		}
	}
	_, cmd := m.handleClusterKey("enter")
	if !m.inspectingNode || !m.nodeJobsLoading || cmd == nil {
		t.Fatal("enter should open details and fetch jobs")
	}
	request := m.nodeJobsRequest
	m.handleClusterKey("esc")
	if m.inspectingNode || m.selectedNode != "a" || m.nodeJobsRequest == request {
		t.Fatal("escape should retain selection and invalidate outstanding fetch")
	}
}

func TestNodeJobsIgnoresStaleResponse(t *testing.T) {
	m := model{
		jobs:        list.New(nil, list.NewDefaultDelegate(), 0, 0),
		clusterView: viewport.New(80, 20), usersView: viewport.New(80, 20),
		selectedNode: "gpu01", inspectingNode: true, nodeJobsRequest: 3,
	}
	for _, msg := range []nodeJobsMsg{
		{node: "gpu02", request: 3, jobs: []slurm.NodeJob{{ID: "wrong-node"}}},
		{node: "gpu01", request: 2, jobs: []slurm.NodeJob{{ID: "old-request"}}},
	} {
		updated, _ := m.Update(msg)
		m = updated.(model)
		if len(m.nodeJobs) != 0 {
			t.Fatal("stale response changed inspected jobs")
		}
	}
	updated, _ := m.Update(nodeJobsMsg{node: "gpu01", request: 3, jobs: []slurm.NodeJob{{ID: "current"}}})
	if got := updated.(model).nodeJobs; len(got) != 1 || got[0].ID != "current" {
		t.Fatalf("current response was not applied: %+v", got)
	}
}

func TestNodeDetailsAllocationScope(t *testing.T) {
	jobs := []slurm.NodeJob{
		{ID: "1", NodeCount: 1, CPUs: "16", TRES: "cpu=16,gres/gpu=1", Elapsed: "01:00:00", Limit: "04:00:00", Remaining: "03:00:00"},
		{ID: "2", NodeCount: 2, CPUs: "128", TRES: "cpu=128,gres/gpu=8", Limit: "UNLIMITED", Remaining: "UNLIMITED"},
	}
	content := stripANSI(renderNodeDetails(slurm.NodeInfo{Name: "gpu01"}, jobs, false, nil, nil, 160))
	for _, want := range []string{"1 GPU", "16 CPU", "Whole job · 2 nodes", "25%", "03:00:00 left", "UNLIMITED", "━"} {
		if !strings.Contains(content, want) {
			t.Errorf("details missing %q:\n%s", want, content)
		}
	}
}

func TestNodePopupFitsTerminal(t *testing.T) {
	for _, size := range []struct{ width, height int }{{240, 80}, {120, 40}, {80, 24}, {60, 20}} {
		m := model{
			width: size.width, height: size.height, inspectingNode: true, selectedNode: "gpu01",
			clusterView: viewport.New(size.width-4, size.height-6), nodeDetailView: viewport.New(0, 0),
			nodes:    []slurm.NodeInfo{{Name: "gpu01", State: "mixed", Partition: "gpu", GPUTotal: 8, GPUUsed: 4, CPUTotal: 128, CPUAlloc: 64, MemTotalMB: 524288, MemFreeMB: 131072, MemAllocMB: 458752}},
			nodeJobs: []slurm.NodeJob{{ID: "123", Name: "training", User: "alice", State: "RUNNING", CPUs: "64", TRES: "cpu=64,mem=128G,gres/gpu=4", NodeCount: 1, Elapsed: "01:00:00", Limit: "04:00:00", Remaining: "03:00:00"}},
		}
		m.updateClusterContent()
		popup := m.nodePopup()
		if lipgloss.Width(popup) != min(110, size.width-6) || lipgloss.Height(popup) > size.height-2 {
			t.Errorf("popup should respect readable width and available height: got %dx%d", lipgloss.Width(popup), lipgloss.Height(popup))
		}
		for _, want := range []string{"RAM", "OS used 384 GiB", "reserved 448 GiB", "512 GiB total"} {
			if !strings.Contains(strings.Join(strings.Fields(stripANSI(popup)), " "), want) {
				t.Errorf("popup missing %q", want)
			}
		}
		card := stripANSI(renderNodeCard(m.nodes[0], false))
		if !strings.Contains(card, "ram 448/512 GiB") {
			t.Errorf("node card missing RAM usage: %s", card)
		}
		if lipgloss.Width(popup) > size.width || lipgloss.Height(popup) > size.height {
			t.Errorf("popup %dx%d exceeds terminal %dx%d:\n%s", lipgloss.Width(popup), lipgloss.Height(popup), size.width, size.height, popup)
		}
		if !strings.Contains(m.clusterView.View(), "gpu01") {
			t.Error("cluster grid must remain behind popup")
		}
		overlay := overlayNodePopup(m.clusterView.View(), popup, size.width, size.height)
		if lipgloss.Width(overlay) != size.width || lipgloss.Height(overlay) != size.height {
			t.Error("overlay dimensions must match terminal")
		}
	}
}

func TestNodePopupHeightFollowsContent(t *testing.T) {
	m := model{
		width: 240, height: 80, inspectingNode: true, selectedNode: "gpu01",
		clusterView: viewport.New(236, 74), nodeDetailView: viewport.New(0, 0),
		nodes: []slurm.NodeInfo{{Name: "gpu01", State: "idle"}},
	}
	m.updateClusterContent()
	emptyHeight := lipgloss.Height(m.nodePopup())
	job := slurm.NodeJob{ID: "123", Name: "training", State: "RUNNING", NodeCount: 1}
	m.nodeJobs = []slurm.NodeJob{job}
	m.updateClusterContent()
	oneHeight := lipgloss.Height(m.nodePopup())
	if oneHeight <= emptyHeight || oneHeight >= m.height-2 {
		t.Fatalf("popup should grow with content: empty=%d, one=%d", emptyHeight, oneHeight)
	}
	if strings.Contains(stripANSI(m.nodePopup()), "↑↓ scroll") {
		t.Error("scroll hint should be hidden when all jobs fit")
	}
	for i := 0; i < 20; i++ {
		m.nodeJobs = append(m.nodeJobs, job)
	}
	m.updateClusterContent()
	if got := lipgloss.Height(m.nodePopup()); got != m.height-2 {
		t.Errorf("many jobs should use available height: got %d, want %d", got, m.height-2)
	}
	if !strings.Contains(stripANSI(m.nodePopup()), "↑↓ scroll") {
		t.Error("overflowing jobs should show scroll hint")
	}
}

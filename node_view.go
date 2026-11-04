package main

import (
	"fmt"
	"slurmy/slurm"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type nodeJobsMsg struct {
	node    string
	request uint64
	jobs    []slurm.NodeJob
	err     error
}

func (m model) fetchNodeJobsCmd() tea.Cmd {
	return func() tea.Msg {
		jobs, err := m.slurmClient.GetNodeJobs(m.selectedNode)
		return nodeJobsMsg{node: m.selectedNode, request: m.nodeJobsRequest, jobs: jobs, err: err}
	}
}

func (m *model) updateClusterContent() {
	m.clusterView.SetContent(renderClusterView(m.nodes, m.clusterSum, m.clusterErr, m.clusterView.Width, m.selectedNode))
	if m.inspectingNode {
		width, height := m.nodePopupSize()
		if m.nodeDetailView.KeyMap.Down.Keys() == nil {
			m.nodeDetailView = viewport.New(0, 0)
		}
		m.nodeDetailView.Width = max(1, width-6)
		content := renderNodeDetails(m.inspectedNode(), m.nodeJobs, m.nodeJobsLoading, m.nodeJobsErr, m.clusterErr, m.nodeDetailView.Width)
		available := max(1, height-lipgloss.Height(m.nodePopupHeader(width-6))-5)
		m.nodeDetailView.Height = min(available, max(1, lipgloss.Height(content)))
		m.nodeDetailView.SetContent(content)
	}
}

func (m *model) handleClusterKey(key string) (bool, tea.Cmd) {
	if m.inspectingNode {
		if key == "esc" || key == "escape" || key == "backspace" {
			m.inspectingNode = false
			m.nodeJobsRequest++
			m.updateClusterContent()
			m.clusterView.SetYOffset(m.gridOffset)
			return true, nil
		}
		return false, nil
	}
	index := 0
	for i, n := range m.nodes {
		if n.Name == m.selectedNode {
			index = i
			break
		}
	}
	perRow := max(1, m.clusterView.Width/24)
	switch key {
	case "enter":
		if len(m.nodes) == 0 {
			return true, nil
		}
		m.selectedNode = m.nodes[index].Name
		m.inspectingNode = true
		m.nodeJobsRequest++
		m.gridOffset = m.clusterView.YOffset
		m.nodeJobs, m.nodeJobsErr = nil, nil
		m.nodeJobsLoading = true
		m.updateClusterContent()
		m.nodeDetailView.GotoTop()
		return true, m.fetchNodeJobsCmd()
	case "left", "h":
		index--
	case "right", "l":
		index++
	case "up", "k":
		index -= perRow
	case "down", "j":
		index += perRow
	case "home", "g":
		index = 0
	case "end", "G":
		index = len(m.nodes) - 1
	default:
		return false, nil
	}
	if len(m.nodes) > 0 {
		index = max(0, min(index, len(m.nodes)-1))
		m.selectedNode = m.nodes[index].Name
		m.updateClusterContent()
		// Summary and legend occupy six lines; each card row is six lines.
		top := 6 + (index/perRow)*6
		if top < m.clusterView.YOffset {
			m.clusterView.SetYOffset(top)
		} else if top+6 > m.clusterView.YOffset+m.clusterView.Height {
			m.clusterView.SetYOffset(top + 6 - m.clusterView.Height)
		}
	}
	return true, nil
}

func renderNodeDetails(node slurm.NodeInfo, jobs []slurm.NodeJob, loading bool, jobsErr, nodeErr error, width int) string {
	var lines []string
	if nodeErr != nil {
		lines = append(lines, nodeMuted.Render("Node refresh failed: "+nodeErr.Error()))
	}
	switch {
	case loading:
		lines = append(lines, nodeMuted.Render("  Loading allocations…"))
	case jobsErr != nil:
		lines = append(lines, lipgloss.NewStyle().Foreground(colAlloc).Render("Could not load jobs: "+jobsErr.Error()))
	case len(jobs) == 0:
		lines = append(lines, "", nodeMuted.Render("  No visible jobs on this node."), "")
	}
	for _, job := range jobs {
		lines = append(lines, renderAllocationCard(job, width), "")
	}
	return lipgloss.NewStyle().Width(max(1, width)).Render(strings.Join(lines, "\n"))
}

var nodeMuted = lipgloss.NewStyle().Foreground(lipgloss.Color("#9299A8"))

func (m model) inspectedNode() slurm.NodeInfo {
	for _, node := range m.nodes {
		if node.Name == m.selectedNode {
			return node
		}
	}
	return slurm.NodeInfo{Name: m.selectedNode}
}

func (m model) nodePopupSize() (int, int) {
	// Keep lines readable on ultrawide terminals. Height is an upper bound;
	// the job viewport shrinks to fit its content when fewer jobs are present.
	return max(12, min(110, m.width-6)), max(10, m.height-2)
}

func (m model) nodePopupHeader(width int) string {
	node := m.inspectedNode()
	bold := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EEF2FA"))
	state := lipgloss.NewStyle().Foreground(stateColor(node.State)).Render("● " + strings.ToUpper(node.State))
	rows := []string{
		popupRow(bold.Render(node.Name), state, width),
		nodeMuted.Render(ansi.Truncate(node.Partition+"  ·  "+displayValue(node.GPUType), width, "…")),
		"",
	}
	if node.GPUTotal > 0 {
		rows = append(rows, resourceMeter("GPU", node.GPUUsed, node.GPUTotal, width))
	}
	rows = append(rows, resourceMeter("CPU", node.CPUAlloc, node.CPUTotal, width))
	rows = append(rows, memoryMeter(node, width))
	rows = append(rows,
		popupRow(bold.Render(fmt.Sprintf("ALLOCATIONS  %d", len(m.nodeJobs))), nodeMuted.Render("↻ live"), width),
		nodeMuted.Render(strings.Repeat("─", max(1, width))))
	return lipgloss.NewStyle().Width(max(1, width)).Render(strings.Join(rows, "\n"))
}

func memoryMeter(node slurm.NodeInfo, width int) string {
	if node.MemTotalMB <= 0 {
		return nodeMuted.Render("RAM  unavailable")
	}
	used, _, known := node.MemoryUsage()
	barWidth := max(1, width-7)
	usedCells, reservedCells := 0, 0
	if known {
		usedCells = min(barWidth, used*barWidth/node.MemTotalMB)
	}
	if node.MemAllocMB >= 0 {
		reservedCells = min(barWidth, node.MemAllocMB*barWidth/node.MemTotalMB)
	}
	bar := lipgloss.NewStyle().Foreground(colMixed).Render(strings.Repeat("━", usedCells)) +
		lipgloss.NewStyle().Foreground(colAlloc).Render(strings.Repeat("━", max(0, reservedCells-usedCells))) +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#343B4C")).Render(strings.Repeat("━", barWidth-max(usedCells, reservedCells)))
	usedLabel, reservedLabel := "OS used —", "reserved —"
	if known {
		usedLabel = fmt.Sprintf("OS used %.0f GiB", float64(used)/1024)
	}
	if node.MemAllocMB >= 0 {
		reservedLabel = fmt.Sprintf("reserved %.0f GiB", float64(node.MemAllocMB)/1024)
	}
	legend := popupRow(lipgloss.NewStyle().Foreground(colMixed).Render(usedLabel), lipgloss.NewStyle().Foreground(colAlloc).Render(reservedLabel), width)
	return nodeMuted.Render("RAM  ") + bar + "\n" + legend + "\n" + nodeMuted.Render(fmt.Sprintf("%.0f GiB total · colours overlap, not additive", float64(node.MemTotalMB)/1024))
}

func resourceMeter(name string, used, total, width int) string {
	percent := 0
	if total > 0 {
		percent = used * 100 / total
	}
	count := fmt.Sprintf("%d / %d", used, total)
	barWidth := max(1, width-len(count)-8)
	return nodeMuted.Render(name+"  ") + allocationBar(percent, barWidth, colMixed) + "  " + count
}

func allocationBar(percent, width int, color lipgloss.Color) string {
	filled := min(width, max(0, percent)*width/100)
	return lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("━", filled)) +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#343B4C")).Render(strings.Repeat("━", width-filled))
}

func renderAllocationCard(job slurm.NodeJob, width int) string {
	inner := max(1, width-4)
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EEF2FA"))
	state := strings.ToLower(job.State)
	stateCol := colMixed
	if job.State != "RUNNING" {
		stateCol = colAlloc
	}
	rows := []string{
		popupRow(title.Render(ansi.Truncate(displayValue(job.Name), max(1, inner-len(state)-3), "…")), lipgloss.NewStyle().Foreground(stateCol).Render("● "+state), inner),
		nodeMuted.Render(ansi.Truncate(job.User+"  ·  #"+job.ID+"  ·  "+job.Partition, inner, "…")),
		"",
	}
	tres := make(map[string]string)
	for _, part := range strings.Split(job.TRES, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok {
			tres[key] = value
		}
	}
	gpu := tres["gres/gpu"]
	if gpu == "" {
		n := 0
		for key, value := range tres {
			if strings.HasPrefix(key, "gres/gpu:") {
				count, _ := strconv.Atoi(value)
				n += count
			}
		}
		gpu = strconv.Itoa(n)
	}
	resources := title.Render(gpu) + nodeMuted.Render(" GPU   ") + title.Render(displayValue(job.CPUs)) + nodeMuted.Render(" CPU")
	if mem := tres["mem"]; mem != "" {
		resources += "   " + title.Render(mem) + nodeMuted.Render(" RAM")
	}
	rows = append(rows, resources)
	if job.NodeCount > 1 {
		rows = append(rows, nodeMuted.Render(fmt.Sprintf("Whole job · %d nodes (per-node split unavailable)", job.NodeCount)))
	} else if job.NodeCount == 0 {
		rows = append(rows, nodeMuted.Render("Whole-job allocation · node count unavailable"))
	}
	rows = append(rows, "")
	if pct, ok := job.TimePercent(); ok {
		color := colMixed
		if pct >= 90 {
			color = colAlloc
		}
		rows = append(rows, allocationBar(pct, max(1, inner-6), color)+title.Render(fmt.Sprintf(" %3d%%", pct)))
	} else {
		rows = append(rows, nodeMuted.Render("Time limit: "+displayValue(job.Limit)))
	}
	rows = append(rows, popupRow(nodeMuted.Render("Elapsed ")+displayValue(job.Elapsed), title.Render(displayValue(job.Remaining))+nodeMuted.Render(" left"), inner))
	if _, ok := job.TimePercent(); ok {
		rows = append(rows, nodeMuted.Render("of "+displayValue(job.Limit)+" wall-time allocation"))
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#343B4C")).
		Padding(0, 1).Width(max(1, width-2)).Render(strings.Join(rows, "\n"))
}

func popupRow(left, right string, width int) string {
	if lipgloss.Width(left)+lipgloss.Width(right)+2 > width {
		return ansi.Truncate(left, width, "…") + "\n" + ansi.Truncate(right, width, "…")
	}
	return left + strings.Repeat(" ", width-lipgloss.Width(left)-lipgloss.Width(right)) + right
}

func (m model) nodePopup() string {
	width, _ := m.nodePopupSize()
	inner := width - 6
	foot := nodeMuted.Render("esc close")
	if m.nodeDetailView.TotalLineCount() > m.nodeDetailView.Height {
		foot = popupRow(nodeMuted.Render("↑↓ scroll  ·  esc close"), nodeMuted.Render(fmt.Sprintf("%d%%", int(m.nodeDetailView.ScrollPercent()*100))), inner)
	}
	content := strings.Join([]string{m.nodePopupHeader(inner), "", m.nodeDetailView.View(), "", foot}, "\n")
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(highlight).
		Foreground(lipgloss.Color("#DEE4EE")).Padding(0, 2).Width(width - 2).Render(content)
}

// Compose by terminal cells so coloured text and wide characters keep their
// alignment, leaving the cluster grid visible around the focused dialog.
func overlayNodePopup(background, popup string, width, height int) string {
	bg := strings.Split(background, "\n")
	fg := strings.Split(popup, "\n")
	pw := lipgloss.Width(popup)
	x, y := max(0, (width-pw)/2), max(0, (height-len(fg))/2)
	var rows []string
	for row := 0; row < height; row++ {
		line := ""
		if row < len(bg) {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("#414959")).Render(ansi.Strip(bg[row]))
		}
		line = ansi.Truncate(line, width, "")
		line += strings.Repeat(" ", max(0, width-lipgloss.Width(line)))
		if row >= y && row-y < len(fg) {
			line = ansi.Cut(line, 0, x) + fg[row-y] + ansi.Cut(line, x+pw, width)
		}
		rows = append(rows, line)
	}
	return strings.Join(rows, "\n")
}

func displayValue(value string) string {
	if value == "" || value == "N/A" || value == "(null)" {
		return "—"
	}
	return value
}

package main

import (
	"fmt"
	"slurmy/slurm"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Tab identifies which view is currently active.
type tab int

const (
	tabJobs tab = iota
	tabCluster
	tabUsers
)

var tabTitles = []string{"Jobs", "Cluster", "Users"}

// tabBarHeight is the number of vertical lines the tab strip plus its spacer
// occupy at the top of every view.
const tabBarHeight = 2

// Node-state colours for the cluster grid.
var (
	colIdle    = lipgloss.Color("#73F59F")
	colMixed   = lipgloss.Color("#08F2CF")
	colAlloc   = lipgloss.Color("#F5A623")
	colDown    = lipgloss.Color("#6B6B6B")
	colCompl   = lipgloss.Color("#7D9CF4")
	colGpuFree = lipgloss.Color("#3A6B4D")
	colGpuPend = lipgloss.Color("#F5A623")
)

// stateColor maps a node's long state string to a display colour.
func stateColor(state string) lipgloss.Color {
	s := strings.ToLower(state)
	switch {
	case strings.Contains(s, "down"), strings.Contains(s, "drain"),
		strings.Contains(s, "maint"), strings.Contains(s, "fail"),
		strings.Contains(s, "unknown"), strings.Contains(s, "boot"),
		strings.Contains(s, "power"), strings.Contains(s, "invalid"):
		return colDown
	case strings.Contains(s, "idle"):
		return colIdle
	case strings.Contains(s, "alloc"):
		return colAlloc
	case strings.Contains(s, "mix"):
		return colMixed
	case strings.Contains(s, "comp"):
		return colCompl
	default:
		return colDown
	}
}

// shortState abbreviates a long node state for the compact node card.
func shortState(state string) string {
	s := strings.ToLower(state)
	suffix := ""
	if strings.HasSuffix(state, "*") {
		suffix = "*"
	}
	switch {
	case strings.Contains(s, "drain"):
		return "DRAIN" + suffix
	case strings.Contains(s, "down"):
		return "DOWN" + suffix
	case strings.Contains(s, "maint"):
		return "MAINT" + suffix
	case strings.Contains(s, "alloc"):
		return "ALLOC" + suffix
	case strings.Contains(s, "mix"):
		return "MIX" + suffix
	case strings.Contains(s, "idle"):
		return "IDLE" + suffix
	case strings.Contains(s, "comp"):
		return "COMPL" + suffix
	default:
		up := strings.ToUpper(state)
		if len(up) > 6 {
			up = up[:6]
		}
		return up
	}
}

// renderTabBar renders the top tab strip with the active tab highlighted.
func renderTabBar(active tab, width int) string {
	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 2).
		Foreground(lipgloss.Color("#1C1C1C")).
		Background(highlight)
	inactiveStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(subtle)

	var tabs []string
	for i, title := range tabTitles {
		label := fmt.Sprintf("%d %s", i+1, title)
		if tab(i) == active {
			tabs = append(tabs, activeStyle.Render(label))
		} else {
			tabs = append(tabs, inactiveStyle.Render(label))
		}
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	hint := lipgloss.NewStyle().Faint(true).Render("  tab/1-3 switch")
	return lipgloss.NewStyle().MarginLeft(1).Render(bar + hint)
}

// gpuBar renders a fixed-width bar of GPU cells: used (coloured), free (green),
// and the remainder (down) dimmed. Used for node cards.
func gpuBar(used, total int, usedColor lipgloss.Color, available bool) string {
	if total <= 0 {
		return lipgloss.NewStyle().Faint(true).Render("no gpu")
	}
	var b strings.Builder
	for i := 0; i < total; i++ {
		switch {
		case i < used:
			b.WriteString(lipgloss.NewStyle().Foreground(usedColor).Render("▰"))
		case available:
			b.WriteString(lipgloss.NewStyle().Foreground(colGpuFree).Render("▱"))
		default:
			b.WriteString(lipgloss.NewStyle().Foreground(colDown).Render("▱"))
		}
	}
	return b.String()
}

// renderNodeCard renders one compact node box.
func renderNodeCard(n slurm.NodeInfo) string {
	col := stateColor(n.State)
	header := lipgloss.NewStyle().Bold(true).Foreground(col).Render(n.Name) +
		" " + lipgloss.NewStyle().Foreground(col).Render(shortState(n.State))

	var midLine string
	if n.HasGPU() {
		midLine = gpuBar(n.GPUUsed, n.GPUTotal, col, n.Available()) +
			lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf(" %d/%d", n.GPUUsed, n.GPUTotal))
	} else {
		midLine = lipgloss.NewStyle().Faint(true).Render("cpu node")
	}

	cpuLine := lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("cpu %d/%d", n.CPUAlloc, n.CPUTotal))

	body := lipgloss.JoinVertical(lipgloss.Left, header, midLine, cpuLine)
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(subtle).
		Padding(0, 1).
		Width(22)
	return border.Render(body)
}

// renderClusterView builds the full cluster tab content (summary + node grid).
func renderClusterView(nodes []slurm.NodeInfo, summary slurm.ClusterSummary, err error, width int) string {
	if err != nil {
		return lipgloss.NewStyle().Foreground(colAlloc).Render(fmt.Sprintf("sinfo error: %v", err))
	}
	if len(nodes) == 0 {
		return "Loading cluster status…"
	}

	label := lipgloss.NewStyle().Bold(true).Foreground(highlight)

	// GPU summary bar (scaled to inner width).
	barWidth := width - 20
	if barWidth < 10 {
		barWidth = 10
	}
	if barWidth > 60 {
		barWidth = 60
	}
	usedCells, freeCells := 0, 0
	if summary.GPUTotal > 0 {
		usedCells = summary.GPUUsed * barWidth / summary.GPUTotal
		freeCells = summary.GPUFree * barWidth / summary.GPUTotal
	}
	if usedCells+freeCells > barWidth {
		freeCells = barWidth - usedCells
	}
	downCells := barWidth - usedCells - freeCells
	gpuMeter := lipgloss.NewStyle().Foreground(colMixed).Render(strings.Repeat("█", usedCells)) +
		lipgloss.NewStyle().Foreground(colIdle).Render(strings.Repeat("█", freeCells)) +
		lipgloss.NewStyle().Foreground(colDown).Render(strings.Repeat("█", downCells))

	pct := 0
	if summary.GPUTotal > 0 {
		pct = summary.GPUUsed * 100 / summary.GPUTotal
	}

	summaryLines := []string{
		label.Render("GPUs ") + gpuMeter,
		fmt.Sprintf("     %s used   %s free   %s down   (of %d, %d%% busy)",
			lipgloss.NewStyle().Foreground(colMixed).Render(fmt.Sprintf("%d", summary.GPUUsed)),
			lipgloss.NewStyle().Foreground(colIdle).Render(fmt.Sprintf("%d", summary.GPUFree)),
			lipgloss.NewStyle().Foreground(colDown).Render(fmt.Sprintf("%d", summary.GPUUnavailable)),
			summary.GPUTotal, pct),
		label.Render("Nodes") + fmt.Sprintf(" %d up · %d down · %d total      ",
			summary.NodesAvailable, summary.NodesDown, summary.NodesTotal) +
			label.Render("CPUs") + fmt.Sprintf(" %d/%d allocated", summary.CPUAlloc, summary.CPUTotal),
	}
	summaryBlock := lipgloss.JoinVertical(lipgloss.Left, summaryLines...)

	// Node grid — only nodes that have GPUs first, then CPU nodes; wrap to width.
	cardWidth := 24 // 22 content + border
	perRow := width / cardWidth
	if perRow < 1 {
		perRow = 1
	}

	var cards []string
	for _, n := range nodes {
		cards = append(cards, renderNodeCard(n))
	}

	var rows []string
	for i := 0; i < len(cards); i += perRow {
		end := i + perRow
		if end > len(cards) {
			end = len(cards)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cards[i:end]...))
	}
	grid := lipgloss.JoinVertical(lipgloss.Left, rows...)

	legend := lipgloss.NewStyle().Faint(true).Render("▰ used   ▱ free   colour = node state")

	return lipgloss.JoinVertical(lipgloss.Left,
		summaryBlock,
		"",
		legend,
		"",
		grid,
	)
}

// renderUsersView builds the users tab: a ranked bar chart of GPU usage.
func renderUsersView(usages []slurm.UserUsage, err error, width int) string {
	if err != nil {
		return lipgloss.NewStyle().Foreground(colAlloc).Render(fmt.Sprintf("squeue error: %v", err))
	}
	if len(usages) == 0 {
		return "Loading user usage…"
	}

	label := lipgloss.NewStyle().Bold(true).Foreground(highlight)

	var totalRun, totalPend int
	maxRun := 1
	for _, u := range usages {
		totalRun += u.RunningGPUs
		totalPend += u.PendingGPUs
		if u.RunningGPUs+u.PendingGPUs > maxRun {
			maxRun = u.RunningGPUs + u.PendingGPUs
		}
	}

	header := label.Render("GPU usage by user") +
		lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("   %d running · %d pending · %d users",
			totalRun, totalPend, len(usages)))

	barMax := width - 34
	if barMax < 10 {
		barMax = 10
	}
	if barMax > 70 {
		barMax = 70
	}

	nameStyle := lipgloss.NewStyle().Width(14)
	var rows []string
	for _, u := range usages {
		runCells := u.RunningGPUs * barMax / maxRun
		pendCells := u.PendingGPUs * barMax / maxRun
		bar := lipgloss.NewStyle().Foreground(colMixed).Render(strings.Repeat("█", runCells)) +
			lipgloss.NewStyle().Foreground(colGpuPend).Render(strings.Repeat("░", pendCells))

		counts := fmt.Sprintf("%2d gpu", u.RunningGPUs)
		if u.PendingGPUs > 0 {
			counts += lipgloss.NewStyle().Foreground(colGpuPend).Render(fmt.Sprintf(" +%d pend", u.PendingGPUs))
		}
		row := nameStyle.Render(u.User) + " " + bar + " " +
			lipgloss.NewStyle().Faint(true).Render(counts)
		rows = append(rows, row)
	}

	legend := lipgloss.NewStyle().Faint(true).Render("█ running   ░ pending")

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		legend,
		"",
		lipgloss.JoinVertical(lipgloss.Left, rows...),
	)
}

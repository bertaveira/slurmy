package main

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"slurmy/slurm"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// separator between job id and name in the plain title text
const jobTitleSep = " / "

// jobDelegate is a custom list delegate for rendering job rows.
// The main point is to basically separate id/name text from the
// state badge so that filter-match highlighting can be applied to
// non-ANSI rendered stuff
type jobDelegate struct {
	styles list.DefaultItemStyles
}

func newJobDelegate() jobDelegate {
	s := list.NewDefaultItemStyles()
	s.SelectedTitle = s.SelectedTitle.Foreground(highlight).BorderLeftForeground(highlight)
	s.SelectedDesc = s.SelectedDesc.Foreground(subtle).BorderLeftForeground(highlight)
	return jobDelegate{styles: s}
}

func (d jobDelegate) Height() int                         { return 2 }
func (d jobDelegate) Spacing() int                        { return 1 }
func (d jobDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d jobDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	job, ok := item.(slurm.JobInfo)
	if !ok || m.Width() <= 0 {
		return
	}
	s := &d.styles

	filterState := m.FilterState()
	isFiltered := filterState == list.Filtering || filterState == list.FilterApplied
	emptyFilter := filterState == list.Filtering && m.FilterValue() == ""

	titleStyle, descStyle := s.NormalTitle, s.NormalDesc
	switch {
		case emptyFilter:
			// Filter box just opened, nothing typed yet
			titleStyle, descStyle = s.DimmedTitle, s.DimmedDesc
		case index == m.Index() && filterState != list.Filtering:
			// row under the cursor, not typing queries
			titleStyle, descStyle = s.SelectedTitle, s.SelectedDesc
	}

	// Plain, ANSI-free text ("<id> / <name>") that filter highlighting can
	// safely style. The badge is added back in afterwards, untouched.
	plain := job.JobID + jobTitleSep + job.JobName
	if isFiltered && !emptyFilter {
		idx := remapFilterMatches(m.MatchesForItem(index), job.JobID, job.JobName)
		unmatched := titleStyle.Inline(true)
		matched := unmatched.Inherit(s.FilterMatch)
		plain = lipgloss.StyleRunes(plain, idx, matched, unmatched)
	}
	title := job.StateBadge() + " " + plain
	desc := job.Description()

	// Prevent text from exceeding the list width
	textwidth := m.Width() - s.NormalTitle.GetPaddingLeft() - s.NormalTitle.GetPaddingRight()
	title = ansi.Truncate(title, textwidth, "…")
	var lines []string
	for i, line := range strings.Split(desc, "\n") {
		if i >= d.Height()-1 {
			break
		}
		lines = append(lines, ansi.Truncate(line, textwidth, "…"))
	}
	desc = strings.Join(lines, "\n")

	title = titleStyle.Render(title)
	desc = descStyle.Render(desc)

	fmt.Fprintf(w, "%s\n%s", title, desc)
}

// remapFilterMatches translates match indices from FilterValue()'s layout
// ("<id> <name> <state>") to plain's layout ("<id> / <name>"). Id indices
// map 1:1; name indices shift by the separator length difference; matches
// in the state (which isn't shown as text) are dropped.
func remapFilterMatches(matches []int, id, name string) []int {
	const sepFilterLen = 1 // single space between id and name in FilterValue
	idLen := utf8.RuneCountInString(id)
	nameLen := utf8.RuneCountInString(name)
	nameStartFilter := idLen + sepFilterLen
	nameStartVisible := idLen + utf8.RuneCountInString(jobTitleSep)

	out := make([]int, 0, len(matches))
	for _, i := range matches {
		switch {
			case i < idLen:
				// highlight in the id part
				out = append(out, i)
			case i >= nameStartFilter && i < nameStartFilter+nameLen:
				// highlight in the name part
				out = append(out, nameStartVisible+(i-nameStartFilter))
		}
	}
	return out
}


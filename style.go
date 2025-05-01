package main

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/gamut"
)

var (
	// General.

	normal        = lipgloss.Color("#EEEEEE")
	normalDark    = lipgloss.Color("#1C1C1C")
	subtle        = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight     = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#08F2CF"}
	softHighlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special       = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	blends        = gamut.Blends(lipgloss.Color("#F25D94"), lipgloss.Color("#EDFF82"), 50)

	base = lipgloss.NewStyle().Foreground(normal)

	divider = lipgloss.NewStyle().
		SetString("•").
		Padding(0, 1).
		Foreground(subtle).
		String()

	url = lipgloss.NewStyle().Foreground(special).Render

	// Title.

	titleStyle = lipgloss.NewStyle().
			MarginLeft(1).
			MarginRight(5).
			Padding(0, 1).
			Italic(true).
			Foreground(lipgloss.Color("#FFF7DB")).
			Background(lipgloss.Color("#F25D94"))

	descStyle = base.MarginTop(1)

	infoStyle = base.
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(subtle)

	// Dialog.

	dialogBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(1, 0).
			BorderTop(true).
			BorderLeft(true).
			BorderRight(true).
			BorderBottom(true)

	buttonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF7DB")).
			Background(lipgloss.Color("#888B7E")).
			Padding(0, 3).
			MarginTop(1)

	activeButtonStyle = buttonStyle.
				Foreground(lipgloss.Color("#FFF7DB")).
				Background(lipgloss.Color("#F25D94")).
				MarginRight(2).
				Underline(true)

	// List.

	listStyle = lipgloss.NewStyle().
			Foreground(normal).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle)

	listHeaderStyle = base.
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(subtle).
			MarginRight(2).
			Render

	listItemStyle = base.PaddingLeft(2).Render

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(normal).
				Background(highlight)

	// Page.

	docStyle = lipgloss.NewStyle().Padding(1, 2, 1, 2)
)

package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

var noColor = os.Getenv("NO_COLOR") != ""

var (
	// Colors
	colorGreen   = lipgloss.Color("#22c55e")
	colorRed     = lipgloss.Color("#ef4444")
	colorCyan    = lipgloss.Color("#06b6d4")
	colorYellow  = lipgloss.Color("#eab308")
	colorDim     = lipgloss.Color("#6b7280")
	colorWhite   = lipgloss.Color("#e5e7eb")
	colorBorder  = lipgloss.Color("#374151")
	colorHighlight = lipgloss.Color("#1e3a5f")

	// Colors - backgrounds
	colorAddedBg   = lipgloss.Color("#0d2818")
	colorRemovedBg = lipgloss.Color("#2d0b0b")

	// Line styles
	addedStyle   = lipgloss.NewStyle().Foreground(colorGreen).Background(colorAddedBg)
	removedStyle = lipgloss.NewStyle().Foreground(colorRed).Background(colorRemovedBg)
	contextStyle = lipgloss.NewStyle().Foreground(colorDim)
	hunkStyle    = lipgloss.NewStyle().Foreground(colorCyan)
	fileStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorWhite)

	// Gutter styles
	addedGutterStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorGreen).Width(11)
	removedGutterStyle = lipgloss.NewStyle().Bold(true).Foreground(colorRed).Width(11)
	contextGutterStyle = lipgloss.NewStyle().Bold(true).Foreground(colorDim).Width(11)

	// File list styles
	selectedStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorWhite)
	unselectedStyle = lipgloss.NewStyle().Foreground(colorDim)

	// Status indicators
	statusModified  = lipgloss.NewStyle().Foreground(colorYellow)
	statusAdded     = lipgloss.NewStyle().Foreground(colorGreen)
	statusDeleted   = lipgloss.NewStyle().Foreground(colorRed)
	statusUntracked = lipgloss.NewStyle().Foreground(colorCyan)

	// Panel styles
	panelBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder)

	focusedBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorCyan)

	// Gutter style
	gutterStyle = lipgloss.NewStyle().Foreground(colorDim).Width(11)

	// Help styles
	helpKeyStyle  = lipgloss.NewStyle().Bold(true).Foreground(colorCyan)
	helpDescStyle = lipgloss.NewStyle().Foreground(colorDim)
	helpStyle     = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorCyan).
			Padding(1, 2)
)

func init() {
	if noColor {
		addedStyle = lipgloss.NewStyle()
		removedStyle = lipgloss.NewStyle()
		contextStyle = lipgloss.NewStyle()
		hunkStyle = lipgloss.NewStyle()
		fileStyle = lipgloss.NewStyle().Bold(true)
		selectedStyle = lipgloss.NewStyle().Bold(true)
		unselectedStyle = lipgloss.NewStyle()
		statusModified = lipgloss.NewStyle()
		statusAdded = lipgloss.NewStyle()
		statusDeleted = lipgloss.NewStyle()
		statusUntracked = lipgloss.NewStyle()
		panelBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
		focusedBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
		gutterStyle = lipgloss.NewStyle().Width(11)
		addedGutterStyle = lipgloss.NewStyle().Bold(true).Width(11)
		removedGutterStyle = lipgloss.NewStyle().Bold(true).Width(11)
		contextGutterStyle = lipgloss.NewStyle().Bold(true).Width(11)
		helpKeyStyle = lipgloss.NewStyle().Bold(true)
		helpDescStyle = lipgloss.NewStyle()
		helpStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
	}
}

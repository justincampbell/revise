package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

var noColor = os.Getenv("NO_COLOR") != ""

var (
	// Colors
	colorGreen  = lipgloss.Color("#22c55e")
	colorRed    = lipgloss.Color("#ef4444")
	colorCyan   = lipgloss.Color("#06b6d4")
	colorYellow = lipgloss.Color("#eab308")
	colorDim    = lipgloss.Color("#6b7280")
	colorWhite  = lipgloss.Color("#e5e7eb")
	colorBorder = lipgloss.Color("#374151")

	// Colors - backgrounds
	colorAddedBg   = lipgloss.Color("#0d2818")
	colorRemovedBg = lipgloss.Color("#2d0b0b")

	// Line styles
	addedStyle        = lipgloss.NewStyle().Foreground(colorGreen).Background(colorAddedBg)
	removedStyle      = lipgloss.NewStyle().Foreground(colorRed).Background(colorRemovedBg)
	contextStyle      = lipgloss.NewStyle().Foreground(colorDim)
	hunkStyle         = lipgloss.NewStyle().Foreground(colorCyan)
	hunkBranchStyle      = lipgloss.NewStyle().Foreground(colorDim)
	hunkStagedStyle      = lipgloss.NewStyle().Foreground(colorCyan)
	hunkUnstagedStyle    = lipgloss.NewStyle().Foreground(colorYellow)
	hunkSourceTagStyle   = lipgloss.NewStyle().Foreground(colorDim).Italic(true)
	fileStyle         = lipgloss.NewStyle().Bold(true).Foreground(colorWhite)

	// Gutter styles
	addedGutterStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorGreen).Width(6)
	removedGutterStyle = lipgloss.NewStyle().Bold(true).Foreground(colorRed).Width(6)
	contextGutterStyle = lipgloss.NewStyle().Bold(true).Foreground(colorDim).Width(6)

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

	// Cursor indicator (1-char prefix column)
	cursorStyle = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)

	// Comment count badge in file list
	commentCountStyle = lipgloss.NewStyle().Foreground(colorYellow)

	// Inline comment display (persisted annotation below a code line)
	commentDisplayStyle = lipgloss.NewStyle().Foreground(colorYellow).Italic(true)

	// Inline comment input box
	commentInputStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorYellow).
				Padding(0, 1)

	// Mode slider styles
	modeActiveStyle   = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	modeInactiveStyle = lipgloss.NewStyle().Foreground(colorDim)

	// Status bar
	statusBarStyle = lipgloss.NewStyle().Foreground(colorDim)

	// Help styles
	helpKeyStyle  = lipgloss.NewStyle().Bold(true).Foreground(colorCyan)
	helpDescStyle = lipgloss.NewStyle().Foreground(colorDim)
	helpStyle     = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorCyan).
			Padding(1, 2)

	// Confirm dialog styles
	confirmStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorRed).
			Padding(1, 2)
	confirmKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(colorRed)
)

func init() {
	if noColor {
		addedStyle = lipgloss.NewStyle()
		removedStyle = lipgloss.NewStyle()
		contextStyle = lipgloss.NewStyle()
		hunkStyle = lipgloss.NewStyle()
		hunkBranchStyle = lipgloss.NewStyle()
		hunkStagedStyle = lipgloss.NewStyle()
		hunkUnstagedStyle = lipgloss.NewStyle()
		hunkSourceTagStyle = lipgloss.NewStyle()
		fileStyle = lipgloss.NewStyle().Bold(true)
		selectedStyle = lipgloss.NewStyle().Bold(true)
		unselectedStyle = lipgloss.NewStyle()
		statusModified = lipgloss.NewStyle()
		statusAdded = lipgloss.NewStyle()
		statusDeleted = lipgloss.NewStyle()
		statusUntracked = lipgloss.NewStyle()
		panelBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
		focusedBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
		addedGutterStyle = lipgloss.NewStyle().Bold(true).Width(6)
		removedGutterStyle = lipgloss.NewStyle().Bold(true).Width(6)
		contextGutterStyle = lipgloss.NewStyle().Bold(true).Width(6)
		cursorStyle = lipgloss.NewStyle().Bold(true)
		commentCountStyle = lipgloss.NewStyle()
		commentDisplayStyle = lipgloss.NewStyle()
		commentInputStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
		modeActiveStyle = lipgloss.NewStyle().Bold(true)
		modeInactiveStyle = lipgloss.NewStyle()
		statusBarStyle = lipgloss.NewStyle()
		helpKeyStyle = lipgloss.NewStyle().Bold(true)
		helpDescStyle = lipgloss.NewStyle()
		helpStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
		confirmStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
		confirmKeyStyle = lipgloss.NewStyle().Bold(true)
	}
}

package ui

import (
	"image/color"
	"os"

	"charm.land/lipgloss/v2"
)

var noColor = os.Getenv("NO_COLOR") != ""

var activeTheme Theme = ThemeAuto
var activeIsDark = true // default until terminal background is detected

// colorBorder, colorCyan, and colorYellow are used directly in diffview.go and help.go
// for dynamic border/style construction.
var (
	colorBorder color.Color = lipgloss.Color("#374151")
	colorCyan   color.Color = lipgloss.Color("#06b6d4")
	colorYellow color.Color = lipgloss.Color("#eab308")
)

// SetTheme sets the active theme. Called from main.go for --theme overrides,
// or with ThemeAuto + detected isDark on startup.
func SetTheme(t Theme, isDark bool) {
	activeTheme = t
	activeIsDark = isDark
	applyTheme(paletteFor(t, isDark))
	if noColor {
		applyNoColor()
	}
	clearHighlightCache()
}

var (
	// Line styles
	addedStyle         lipgloss.Style
	removedStyle       lipgloss.Style
	contextStyle       lipgloss.Style
	hunkStyle          lipgloss.Style
	hunkBranchStyle    lipgloss.Style
	hunkStagedStyle    lipgloss.Style
	hunkUnstagedStyle  lipgloss.Style
	hunkSourceTagStyle lipgloss.Style
	fileStyle          lipgloss.Style

	// Gutter styles
	addedGutterStyle   lipgloss.Style
	removedGutterStyle lipgloss.Style
	contextGutterStyle lipgloss.Style

	// File list styles
	selectedStyle   lipgloss.Style
	unselectedStyle lipgloss.Style

	// Status indicators
	statusModified        lipgloss.Style
	statusAdded           lipgloss.Style
	statusDeleted         lipgloss.Style
	statusUntracked       lipgloss.Style
	statusPartiallyStaged lipgloss.Style

	// Dim variants for branch-only (committed) indicators
	statusDimModified lipgloss.Style
	statusDimAdded    lipgloss.Style
	statusDimDeleted  lipgloss.Style
	statusDimRenamed  lipgloss.Style

	// Panel styles
	panelBorder   lipgloss.Style
	focusedBorder lipgloss.Style

	// Cursor indicator (1-char prefix column)
	cursorStyle lipgloss.Style

	// Comment/mark count badges in file list
	commentCountStyle lipgloss.Style
	markCountStyle    lipgloss.Style

	// Inline comment display (persisted annotation below a code line)
	commentDisplayStyle lipgloss.Style

	// Marked line styles
	markAddedStyle    lipgloss.Style
	markRemovedStyle  lipgloss.Style
	markContextStyle  lipgloss.Style
	markGutterAdded   lipgloss.Style
	markGutterRemoved lipgloss.Style
	markGutterContext lipgloss.Style

	// Inline comment input box
	commentInputStyle lipgloss.Style

	// Mode slider styles
	modeActiveStyle   lipgloss.Style
	modeInactiveStyle lipgloss.Style
	modeDisabledStyle lipgloss.Style // mode not available on current branch

	// Status bar
	statusBarStyle lipgloss.Style

	// Help styles
	helpKeyStyle   lipgloss.Style
	helpTextStyle  lipgloss.Style
	helpGroupStyle lipgloss.Style
	helpDescStyle  lipgloss.Style
	helpStyle      lipgloss.Style

	// Confirm dialog styles
	confirmStyle    lipgloss.Style
	confirmKeyStyle lipgloss.Style
)

func init() {
	applyTheme(paletteFor(activeTheme, activeIsDark))
	if noColor {
		applyNoColor()
	}
}

func applyTheme(p themeColors) {
	colorBorder = p.border
	colorCyan = p.cyan
	colorYellow = p.yellow

	addedStyle = lipgloss.NewStyle().Foreground(p.addedFg).Background(p.addedBg)
	removedStyle = lipgloss.NewStyle().Foreground(p.removedFg).Background(p.removedBg)
	contextStyle = lipgloss.NewStyle().Foreground(p.dim)
	hunkStyle = lipgloss.NewStyle().Foreground(p.cyan)
	hunkBranchStyle = lipgloss.NewStyle().Foreground(p.dim)
	hunkStagedStyle = lipgloss.NewStyle().Foreground(p.cyan)
	hunkUnstagedStyle = lipgloss.NewStyle().Foreground(p.yellow)
	hunkSourceTagStyle = lipgloss.NewStyle().Foreground(p.dim).Italic(true)
	fileStyle = lipgloss.NewStyle().Bold(true).Foreground(p.white)

	gutterAdded := p.gutterAddedFg
	if gutterAdded == nil {
		gutterAdded = p.addedFg
	}
	gutterRemoved := p.gutterRemovedFg
	if gutterRemoved == nil {
		gutterRemoved = p.removedFg
	}
	addedGutterStyle = lipgloss.NewStyle().Bold(true).Foreground(gutterAdded).Width(6)
	removedGutterStyle = lipgloss.NewStyle().Bold(true).Foreground(gutterRemoved).Width(6)
	contextGutterStyle = lipgloss.NewStyle().Bold(true).Foreground(p.dim).Width(6)

	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(p.white)
	unselectedStyle = lipgloss.NewStyle().Foreground(p.dim)

	statusModified = lipgloss.NewStyle().Foreground(p.yellow)
	statusAdded = lipgloss.NewStyle().Foreground(p.addedFg)
	statusDeleted = lipgloss.NewStyle().Foreground(p.removedFg)
	statusUntracked = lipgloss.NewStyle().Foreground(p.cyan)
	statusPartiallyStaged = lipgloss.NewStyle().Foreground(p.cyan)

	statusDimModified = lipgloss.NewStyle().Foreground(p.dimYellow)
	statusDimAdded = lipgloss.NewStyle().Foreground(p.dimGreen)
	statusDimDeleted = lipgloss.NewStyle().Foreground(p.dimRed)
	statusDimRenamed = lipgloss.NewStyle().Foreground(p.dimYellow)

	panelBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.border)

	focusedBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.cyan)

	cursorStyle = lipgloss.NewStyle().Foreground(p.cyan).Bold(true)

	commentCountStyle = lipgloss.NewStyle().Foreground(p.yellow)
	markCountStyle = lipgloss.NewStyle().Foreground(p.cyan)
	commentDisplayStyle = lipgloss.NewStyle().Foreground(p.yellow).Italic(true)

	markAddedStyle = lipgloss.NewStyle().Foreground(p.addedFg).Background(p.markBg)
	markRemovedStyle = lipgloss.NewStyle().Foreground(p.removedFg).Background(p.markBg)
	markContextStyle = lipgloss.NewStyle().Foreground(p.white).Background(p.markBg)
	markGutterAdded = lipgloss.NewStyle().Bold(true).Foreground(p.addedFg).Background(p.markBg).Width(6)
	markGutterRemoved = lipgloss.NewStyle().Bold(true).Foreground(p.removedFg).Background(p.markBg).Width(6)
	markGutterContext = lipgloss.NewStyle().Bold(true).Foreground(p.white).Background(p.markBg).Width(6)

	commentInputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.yellow).
		Padding(0, 1)

	modeActiveStyle = lipgloss.NewStyle().Foreground(p.cyan).Bold(true)
	modeInactiveStyle = lipgloss.NewStyle().Foreground(p.dim)
	modeDisabledStyle = lipgloss.NewStyle().Foreground(p.dim).Faint(true).Strikethrough(true)

	statusBarStyle = lipgloss.NewStyle().Foreground(p.dim)

	helpKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(p.cyan)
	helpTextStyle = lipgloss.NewStyle().Foreground(p.white)
	helpGroupStyle = lipgloss.NewStyle().Bold(true).Foreground(p.white)
	helpDescStyle = lipgloss.NewStyle().Foreground(p.dim)
	helpStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.yellow).
		Padding(1, 2, 0)

	confirmStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.removedFg).
		Padding(1, 2)
	confirmKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(p.removedFg)
}

func applyNoColor() {
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
	statusPartiallyStaged = lipgloss.NewStyle()
	statusDimModified = lipgloss.NewStyle()
	statusDimAdded = lipgloss.NewStyle()
	statusDimDeleted = lipgloss.NewStyle()
	statusDimRenamed = lipgloss.NewStyle()
	panelBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	focusedBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	addedGutterStyle = lipgloss.NewStyle().Bold(true).Width(6)
	removedGutterStyle = lipgloss.NewStyle().Bold(true).Width(6)
	contextGutterStyle = lipgloss.NewStyle().Bold(true).Width(6)
	cursorStyle = lipgloss.NewStyle().Bold(true)
	commentCountStyle = lipgloss.NewStyle()
	markCountStyle = lipgloss.NewStyle()
	commentDisplayStyle = lipgloss.NewStyle()
	markAddedStyle = lipgloss.NewStyle().Reverse(true)
	markRemovedStyle = lipgloss.NewStyle().Reverse(true)
	markContextStyle = lipgloss.NewStyle().Reverse(true)
	markGutterAdded = lipgloss.NewStyle().Bold(true).Reverse(true).Width(6)
	markGutterRemoved = lipgloss.NewStyle().Bold(true).Reverse(true).Width(6)
	markGutterContext = lipgloss.NewStyle().Bold(true).Reverse(true).Width(6)
	commentInputStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	modeActiveStyle = lipgloss.NewStyle().Bold(true)
	modeInactiveStyle = lipgloss.NewStyle()
	modeDisabledStyle = lipgloss.NewStyle().Faint(true).Strikethrough(true)
	statusBarStyle = lipgloss.NewStyle()
	helpKeyStyle = lipgloss.NewStyle().Bold(true)
	helpTextStyle = lipgloss.NewStyle()
	helpGroupStyle = lipgloss.NewStyle().Bold(true)
	helpDescStyle = lipgloss.NewStyle()
	helpStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2, 0)
	confirmStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
	confirmKeyStyle = lipgloss.NewStyle().Bold(true)
}

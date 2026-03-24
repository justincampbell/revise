package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// helpLineCount returns the total number of lines in the help content.
func helpLineCount() int {
	n := 0
	for _, group := range allBindings {
		n += 2 // blank line + group name
		n += len(group.Bindings)
	}
	return n
}

// helpMaxScroll returns the maximum scroll offset for the help overlay.
func helpMaxScroll(height int) int {
	maxLines := helpVisibleLines(height)
	max := helpLineCount() - maxLines
	if max < 0 {
		return 0
	}
	return max
}

// helpVisibleLines returns the number of content lines visible in the help box.
func helpVisibleLines(height int) int {
	borderRows := 2    // top + bottom border
	paddingRows := 1   // top padding only (PaddingTop(1))
	indicatorRows := 2 // top + bottom scroll indicator slots
	chrome := borderRows + paddingRows + indicatorRows
	maxLines := height - chrome
	if maxLines < 3 {
		maxLines = 3
	}
	return maxLines
}

func renderHelp(width, height, scroll int) string {
	var lines []string

	// Compute max content width from raw text (before styling) so ANSI
	// escape sequences don't inflate the measurement.
	maxW := 0
	for _, group := range allBindings {
		if w := len([]rune(group.Name)); w > maxW {
			maxW = w
		}
		for _, bind := range group.Bindings {
			// "  " + key(14) + desc
			w := 2 + 14 + len([]rune(bind.Desc))
			if w > maxW {
				maxW = w
			}
		}
	}
	for _, group := range allBindings {
		lines = append(lines, "")
		lines = append(lines, helpGroupStyle.Render(group.Name))
		for _, bind := range group.Bindings {
			key := helpKeyStyle.Render(padRight(bind.Key, 14))
			desc := helpTextStyle.Render(bind.Desc)
			lines = append(lines, "  "+key+desc)
		}
	}

	maxLines := helpVisibleLines(height)

	// Clamp scroll
	if scroll > len(lines)-maxLines {
		scroll = len(lines) - maxLines
	}
	if scroll < 0 {
		scroll = 0
	}

	end := scroll + maxLines
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[scroll:end]

	// Build output with dedicated scroll indicator rows.
	var parts []string
	if scroll > 0 {
		parts = append(parts, helpDescStyle.Render("↑ scroll up"))
	} else {
		parts = append(parts, "")
	}
	parts = append(parts, visible...)
	if end < len(lines) {
		parts = append(parts, helpDescStyle.Render("↓ scroll down"))
	} else {
		parts = append(parts, "")
	}

	content := strings.Join(parts, "\n")

	// lipgloss Width/Height include padding but exclude borders.
	paddingLR := 2 + 2          // PaddingLeft(2) + PaddingRight(2)
	paddingTop := 1             // PaddingTop(1)
	contentRows := maxLines + 2 // visible lines + top/bottom indicator slots
	boxWidth := maxW + paddingLR
	boxHeight := contentRows + paddingTop

	rendered := helpStyle.Width(boxWidth).Height(boxHeight).Render(content)

	// Overlay "Keyboard Shortcuts" title onto the top border.
	rendered = setHelpBorderTitle(rendered, "Keyboard Shortcuts")

	return rendered
}

// setHelpBorderTitle overlays a title onto the top border of the help box.
func setHelpBorderTitle(rendered, title string) string {
	nl := strings.IndexByte(rendered, '\n')
	if nl < 0 {
		return rendered
	}
	topLine := rendered[:nl]
	rest := rendered[nl:]

	bc := lipgloss.NewStyle().Foreground(colorYellow)
	titleRendered := fileStyle.Render(title)
	titleWidth := ansi.StringWidth(titleRendered)
	totalWidth := ansi.StringWidth(topLine)

	if titleWidth+2 > totalWidth {
		return rendered
	}

	fillWidth := totalWidth - 2 - titleWidth // minus ╭ and ╮
	if fillWidth < 0 {
		fillWidth = 0
	}
	leftFill := fillWidth / 2
	rightFill := fillWidth - leftFill

	newTop := bc.Render("╭") + bc.Render(strings.Repeat("─", leftFill)) + titleRendered + bc.Render(strings.Repeat("─", rightFill)) + bc.Render("╮")
	return newTop + rest
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

func padRight(s string, n int) string {
	l := len([]rune(s))
	if l >= n {
		return s
	}
	return s + strings.Repeat(" ", n-l)
}

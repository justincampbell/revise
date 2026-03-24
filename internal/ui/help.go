package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderHelp(width, height, scroll int) string {
	var lines []string

	lines = append(lines, fileStyle.Render("Keyboard Shortcuts"))

	for _, group := range allBindings {
		lines = append(lines, "")
		lines = append(lines, helpDescStyle.Render(group.Name))
		for _, bind := range group.Bindings {
			key := helpKeyStyle.Render(padRight(bind.Key, 14))
			desc := helpDescStyle.Render(bind.Desc)
			lines = append(lines, "  "+key+desc)
		}
	}

	// Available height inside the help box (border=2 + padding=2)
	maxLines := height - 6
	if maxLines < 3 {
		maxLines = 3
	}

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

	// Add scroll indicators
	if scroll > 0 {
		visible[0] = helpDescStyle.Render("↑ scroll up") + "  " + visible[0]
	}
	if end < len(lines) {
		visible = append(visible, helpDescStyle.Render("↓ scroll down"))
	}

	content := strings.Join(visible, "\n")

	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		helpStyle.Render(content),
	)
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

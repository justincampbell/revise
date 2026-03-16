package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderHelp(width, height int) string {
	var b strings.Builder

	b.WriteString(fileStyle.Render("Keyboard Shortcuts"))
	b.WriteString("\n")

	for _, group := range allBindings {
		b.WriteString("\n")
		b.WriteString(helpDescStyle.Render(group.Name) + "\n")
		for _, bind := range group.Bindings {
			key := helpKeyStyle.Render(padRight(bind.Key, 14))
			desc := helpDescStyle.Render(bind.Desc)
			b.WriteString("  " + key + desc + "\n")
		}
	}

	content := b.String()

	// Trim content to fit within terminal height if needed.
	// Reserve space for help box border (2) + padding (2) + outer margin (2).
	maxContentLines := height - 6
	if maxContentLines < 5 {
		maxContentLines = 5
	}
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(lines) > maxContentLines {
		lines = lines[:maxContentLines]
		lines = append(lines, helpDescStyle.Render("  (scroll down for more — resize terminal)"))
		content = strings.Join(lines, "\n") + "\n"
	}

	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		helpStyle.Render(content),
	)
}

func padRight(s string, n int) string {
	l := len([]rune(s))
	if l >= n {
		return s
	}
	return s + strings.Repeat(" ", n-l)
}

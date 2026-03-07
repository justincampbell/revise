package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/justincampbell/revise/internal/git"
)

type diffViewModel struct {
	file   *git.FileDiff
	lines  []string // pre-rendered lines
	offset int      // scroll offset
	height int
	width  int
}

func newDiffViewModel() diffViewModel {
	return diffViewModel{}
}

func (m *diffViewModel) setFile(f *git.FileDiff) {
	m.file = f
	m.offset = 0
	m.lines = m.renderLines()
}

func (m *diffViewModel) scrollUp(n int) {
	m.offset -= n
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m *diffViewModel) scrollDown(n int) {
	m.offset += n
	max := len(m.lines) - m.viewHeight()
	if max < 0 {
		max = 0
	}
	if m.offset > max {
		m.offset = max
	}
}

func (m *diffViewModel) goToTop() {
	m.offset = 0
}

func (m *diffViewModel) goToBottom() {
	max := len(m.lines) - m.viewHeight()
	if max < 0 {
		max = 0
	}
	m.offset = max
}

func (m *diffViewModel) halfPageDown() {
	m.scrollDown(m.viewHeight() / 2)
}

func (m *diffViewModel) halfPageUp() {
	m.scrollUp(m.viewHeight() / 2)
}

func (m *diffViewModel) pageDown() {
	m.scrollDown(m.viewHeight())
}

func (m *diffViewModel) pageUp() {
	m.scrollUp(m.viewHeight())
}

func (m *diffViewModel) viewHeight() int {
	h := m.height - 2 // border
	if h < 1 {
		h = 1
	}
	return h
}

func (m diffViewModel) renderLines() []string {
	if m.file == nil {
		return []string{"No file selected"}
	}

	var lines []string

	// File header
	header := m.file.Path
	if m.file.OldPath != "" {
		header = m.file.OldPath + " → " + m.file.Path
	}
	lines = append(lines, fileStyle.Render(header))
	lines = append(lines, "")

	for _, hunk := range m.file.Hunks {
		lines = append(lines, hunkStyle.Render(hunk.Header))

		for _, line := range hunk.Lines {
			lines = append(lines, renderDiffLine(line))
		}
		lines = append(lines, "")
	}

	return lines
}

func renderDiffLine(l git.Line) string {
	gutter := formatGutter(l)
	switch l.Type {
	case git.LineAdded:
		return addedGutterStyle.Render(gutter) + addedStyle.Render(l.Content)
	case git.LineRemoved:
		return removedGutterStyle.Render(gutter) + removedStyle.Render(l.Content)
	case git.LineContext:
		return contextGutterStyle.Render(gutter) + contextStyle.Render(l.Content)
	}
	return l.Content
}

func formatGutter(l git.Line) string {
	switch l.Type {
	case git.LineAdded:
		return fmt.Sprintf("     %4d ", l.NewNum)
	case git.LineRemoved:
		return fmt.Sprintf("%4d      ", l.OldNum)
	case git.LineContext:
		return fmt.Sprintf("%4d %4d ", l.OldNum, l.NewNum)
	}
	return "          "
}

func (m diffViewModel) render(focused bool) string {
	if len(m.lines) == 0 {
		return panelBorder.Width(m.width).Height(m.height).Render("No changes to display")
	}

	viewH := m.viewHeight()
	end := m.offset + viewH
	if end > len(m.lines) {
		end = len(m.lines)
	}

	visible := m.lines[m.offset:end]

	// Truncate long lines to prevent wrapping
	maxWidth := m.width - 2 // border padding
	for i, line := range visible {
		if ansi.StringWidth(line) > maxWidth {
			visible[i] = ansi.Truncate(line, maxWidth, "")
		}
	}

	content := strings.Join(visible, "\n")

	// Scroll indicator
	scrollInfo := ""
	if len(m.lines) > viewH {
		pct := 0
		if len(m.lines)-viewH > 0 {
			pct = m.offset * 100 / (len(m.lines) - viewH)
		}
		scrollInfo = fmt.Sprintf(" %d%% ", pct)
	}
	_ = scrollInfo // TODO: integrate into border

	style := panelBorder
	if focused {
		style = focusedBorder
	}
	return style.Width(m.width).Height(m.height).MaxHeight(m.height + 2).Render(content)
}

package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/x/ansi"
	"github.com/justincampbell/revise/internal/git"
)

// lineRef tracks the source line metadata for a rendered display line.
// It is nil for non-code lines (file header, hunk header, blank separators).
type lineRef struct {
	newNum   int
	oldNum   int
	lineType git.LineType
}

// commentLineNum returns the line number used as the comment key.
// Removed lines use old line numbers; all others use new line numbers.
func (r *lineRef) commentLineNum() int {
	if r.lineType == git.LineRemoved {
		return r.oldNum
	}
	return r.newNum
}

type diffViewModel struct {
	file     *git.FileDiff
	lines    []string   // pre-rendered display lines
	lineRefs []*lineRef // parallel to lines; nil for non-code lines
	cursor   int        // absolute index into lines[]
	offset   int        // scroll offset
	height   int
	width    int
	comments comments

	commentInputActive bool
	textInput          textinput.Model
}

func newDiffViewModel() diffViewModel {
	ti := textinput.New()
	ti.Placeholder = "Add a comment…"
	ti.CharLimit = 500
	return diffViewModel{
		comments:  make(comments),
		textInput: ti,
	}
}

func (m *diffViewModel) setFile(f *git.FileDiff) {
	m.file = f
	m.cursor = 0
	m.offset = 0
	m.buildLines()
}

func (m *diffViewModel) buildLines() {
	m.lines = nil
	m.lineRefs = nil

	if m.file == nil {
		m.lines = []string{"No file selected"}
		m.lineRefs = []*lineRef{nil}
		return
	}

	add := func(s string, ref *lineRef) {
		m.lines = append(m.lines, s)
		m.lineRefs = append(m.lineRefs, ref)
	}

	header := m.file.Path
	if m.file.OldPath != "" {
		header = m.file.OldPath + " → " + m.file.Path
	}
	add(fileStyle.Render(header), nil)
	add("", nil)

	for _, hunk := range m.file.Hunks {
		add(hunkStyle.Render(hunk.Header), nil)
		for _, line := range hunk.Lines {
			ref := &lineRef{
				newNum:   line.NewNum,
				oldNum:   line.OldNum,
				lineType: line.Type,
			}
			add(renderDiffLine(line), ref)
		}
		add("", nil)
	}
}

func (m *diffViewModel) cursorRef() *lineRef {
	if m.cursor < 0 || m.cursor >= len(m.lineRefs) {
		return nil
	}
	return m.lineRefs[m.cursor]
}

func (m *diffViewModel) moveCursorDown(n int) {
	m.cursor += n
	if len(m.lines) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.lines) {
		m.cursor = len(m.lines) - 1
	}
	viewH := m.viewHeight()
	if m.cursor >= m.offset+viewH {
		m.offset = m.cursor - viewH + 1
	}
}

func (m *diffViewModel) moveCursorUp(n int) {
	m.cursor -= n
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
}

func (m *diffViewModel) clampCursorToView() {
	viewH := m.viewHeight()
	if m.cursor < m.offset {
		m.cursor = m.offset
	}
	if m.cursor >= m.offset+viewH {
		m.cursor = m.offset + viewH - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *diffViewModel) scrollUp(n int) {
	m.offset -= n
	if m.offset < 0 {
		m.offset = 0
	}
	m.clampCursorToView()
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
	m.clampCursorToView()
}

func (m *diffViewModel) goToTop() {
	m.offset = 0
	m.cursor = 0
}

func (m *diffViewModel) goToBottom() {
	max := len(m.lines) - m.viewHeight()
	if max < 0 {
		max = 0
	}
	m.offset = max
	m.cursor = len(m.lines) - 1
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *diffViewModel) halfPageDown() {
	m.moveCursorDown(m.viewHeight() / 2)
}

func (m *diffViewModel) halfPageUp() {
	m.moveCursorUp(m.viewHeight() / 2)
}

func (m *diffViewModel) pageDown() {
	m.moveCursorDown(m.viewHeight())
}

func (m *diffViewModel) pageUp() {
	m.moveCursorUp(m.viewHeight())
}

func (m *diffViewModel) viewHeight() int {
	h := m.height - 2 // border
	if h < 1 {
		h = 1
	}
	return h
}

func (m diffViewModel) renderLines() []string {
	return m.lines
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

// linePrefix returns the 1-character prefix for a display line at the given absolute index.
func (m diffViewModel) linePrefix(absIdx int) string {
	isCursor := absIdx == m.cursor
	isCode := absIdx < len(m.lineRefs) && m.lineRefs[absIdx] != nil
	hasComment := false
	if isCode && m.file != nil {
		ref := m.lineRefs[absIdx]
		key := commentKey{file: m.file.Path, lineNum: ref.commentLineNum()}
		_, hasComment = m.comments[key]
	}

	switch {
	case isCursor && hasComment:
		return commentCursorStyle.Render("◆")
	case isCursor:
		return cursorStyle.Render("▶")
	case hasComment:
		return commentMarkerStyle.Render("◆")
	default:
		return " "
	}
}

// inputBoxHeight is the number of rows the inline comment input box occupies.
const inputBoxHeight = 3 // border top + content + border bottom

func (m diffViewModel) render(focused bool) string {
	if len(m.lines) == 0 {
		return panelBorder.Width(m.width).Height(m.height).Render("No changes to display")
	}

	viewH := m.viewHeight()
	// maxWidth: subtract panel border (2) and cursor prefix char (1)
	maxWidth := m.width - 3
	if maxWidth < 1 {
		maxWidth = 1
	}

	renderLine := func(absIdx int) string {
		line := m.lines[absIdx]
		if ansi.StringWidth(line) > maxWidth {
			line = ansi.Truncate(line, maxWidth, "")
		}
		return m.linePrefix(absIdx) + line
	}

	var renderedLines []string

	if m.commentInputActive {
		// Rows above and including cursor line
		codeAbove := m.cursor - m.offset + 1
		if codeAbove < 0 {
			codeAbove = 0
		}
		// Rows available for code below input box
		codeBelow := viewH - inputBoxHeight - codeAbove
		if codeBelow < 0 {
			codeBelow = 0
		}

		// Code lines up to and including cursor
		end := m.offset + codeAbove
		if end > len(m.lines) {
			end = len(m.lines)
		}
		for absIdx := m.offset; absIdx < end; absIdx++ {
			renderedLines = append(renderedLines, renderLine(absIdx))
		}

		// Inline input box (width fits inside panel border)
		inputWidth := m.width - 4
		if inputWidth < 10 {
			inputWidth = 10
		}
		m.textInput.Width = inputWidth - 4 // account for border + padding
		inputBox := commentInputStyle.Width(inputWidth).Render(m.textInput.View())
		renderedLines = append(renderedLines, inputBox)

		// Code lines after cursor
		startAfter := m.cursor + 1
		endAfter := startAfter + codeBelow
		if endAfter > len(m.lines) {
			endAfter = len(m.lines)
		}
		for absIdx := startAfter; absIdx < endAfter; absIdx++ {
			renderedLines = append(renderedLines, renderLine(absIdx))
		}
	} else {
		end := m.offset + viewH
		if end > len(m.lines) {
			end = len(m.lines)
		}
		for absIdx := m.offset; absIdx < end; absIdx++ {
			renderedLines = append(renderedLines, renderLine(absIdx))
		}
	}

	content := strings.Join(renderedLines, "\n")

	style := panelBorder
	if focused {
		style = focusedBorder
	}
	return style.Width(m.width).Height(m.height).MaxHeight(m.height + 2).Render(content)
}

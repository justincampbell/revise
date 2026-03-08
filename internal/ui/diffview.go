package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/justincampbell/revise/internal/git"
)

// lineRef tracks the source line metadata for a rendered display line.
// isCommentDisplay is true for comment annotation lines inserted below code lines.
// isHunkHeader is true for hunk header lines (non-navigable, styled in render).
// nil is used for blank separators.
type lineRef struct {
	newNum           int
	oldNum           int
	lineType         git.LineType
	isCommentDisplay bool
	isHunkHeader     bool
}

// commentKey returns the storage key for a comment on this line.
// Removed lines are keyed by old line number; all others by new line number.
func (r *lineRef) commentKey(file string) commentKey {
	if r.lineType == git.LineRemoved {
		return commentKey{file: file, lineNum: r.oldNum, isOld: true}
	}
	return commentKey{file: file, lineNum: r.newNum}
}

type diffViewModel struct {
	file     *git.FileDiff
	lines    []string   // pre-rendered display lines
	lineRefs []*lineRef // parallel to lines
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
	m.goToFirstNavigable()
}

func (m *diffViewModel) buildLines() {
	m.lines = nil
	m.lineRefs = nil

	if m.file == nil {
		return
	}

	add := func(s string, ref *lineRef) {
		m.lines = append(m.lines, s)
		m.lineRefs = append(m.lineRefs, ref)
	}

	for _, hunk := range m.file.Hunks {
		add(formatHunkHeader(hunk), &lineRef{isHunkHeader: true})
		for _, line := range hunk.Lines {
			ref := &lineRef{
				newNum:   line.NewNum,
				oldNum:   line.OldNum,
				lineType: line.Type,
			}
			add(renderDiffLine(line), ref)

			// If this code line has a saved comment, add a display line below it.
			key := ref.commentKey(m.file.Path)
			if text, ok := m.comments[key]; ok {
				displayRef := &lineRef{isCommentDisplay: true}
				add(commentDisplayStyle.Render("  ╰ "+text), displayRef)
			}
		}
		add("", nil)
	}
}

// formatHunkHeader extracts the function context from a raw "@@ ... @@ context" header.
// Falls back to "@@ line N" when no context is present.
func formatHunkHeader(h git.Hunk) string {
	parts := strings.SplitN(h.Header, "@@", 3)
	if len(parts) == 3 {
		context := strings.TrimSpace(parts[2])
		if context != "" {
			return context
		}
	}
	return fmt.Sprintf("@@ line %d", h.NewStart)
}

func (m diffViewModel) renderFileHeader() string {
	var name string
	if m.file == nil {
		name = "No file selected"
	} else if m.file.OldPath != "" {
		name = m.file.OldPath + " → " + m.file.Path
	} else {
		name = m.file.Path
	}
	return fileHeaderStyle.Render(" " + name)
}

// rebuildLinesPreservingCursor rebuilds display lines and restores the cursor
// to the same code line after the rebuild (handles index shifts from added/removed
// comment display lines).
func (m *diffViewModel) rebuildLinesPreservingCursor() {
	var saved *lineRef
	if m.cursor >= 0 && m.cursor < len(m.lineRefs) {
		ref := m.lineRefs[m.cursor]
		if ref != nil && !ref.isCommentDisplay && !ref.isHunkHeader {
			saved = ref
		}
	}

	m.buildLines()

	if saved == nil {
		return
	}
	for i, ref := range m.lineRefs {
		if ref != nil && !ref.isCommentDisplay && !ref.isHunkHeader &&
			ref.newNum == saved.newNum &&
			ref.oldNum == saved.oldNum &&
			ref.lineType == saved.lineType {
			m.cursor = i
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
			viewH := m.viewHeight()
			if m.cursor >= m.offset+viewH {
				m.offset = m.cursor - viewH + 1
			}
			return
		}
	}
}

// isNavigable reports whether the line at idx can receive the cursor.
// Only code lines (non-nil, non-comment-display, non-hunk-header) are navigable.
func (m *diffViewModel) isNavigable(idx int) bool {
	if idx < 0 || idx >= len(m.lineRefs) {
		return false
	}
	ref := m.lineRefs[idx]
	return ref != nil && !ref.isCommentDisplay && !ref.isHunkHeader
}

func (m *diffViewModel) isCommentDisplayLine(idx int) bool {
	if idx < 0 || idx >= len(m.lineRefs) {
		return false
	}
	ref := m.lineRefs[idx]
	return ref != nil && ref.isCommentDisplay
}

func (m *diffViewModel) cursorRef() *lineRef {
	if !m.isNavigable(m.cursor) {
		return nil
	}
	return m.lineRefs[m.cursor]
}

// goToFirstNavigable positions the cursor on the first navigable (code) line.
func (m *diffViewModel) goToFirstNavigable() {
	for m.cursor < len(m.lineRefs) && !m.isNavigable(m.cursor) {
		m.cursor++
	}
	if m.cursor >= len(m.lineRefs) {
		m.cursor = 0
	}
}

// goToLastNavigable positions the cursor on the last navigable (code) line.
func (m *diffViewModel) goToLastNavigable() {
	m.cursor = len(m.lines) - 1
	for m.cursor > 0 && !m.isNavigable(m.cursor) {
		m.cursor--
	}
}

func (m *diffViewModel) moveCursorDown(n int) {
	for n > 0 && m.cursor < len(m.lines)-1 {
		m.cursor++
		if m.isNavigable(m.cursor) {
			n--
		}
	}
	// If loop ended on a non-navigable line, step back to the last navigable one.
	for !m.isNavigable(m.cursor) && m.cursor > 0 {
		m.cursor--
	}
	viewH := m.viewHeight()
	if m.cursor >= m.offset+viewH {
		m.offset = m.cursor - viewH + 1
	}
}

func (m *diffViewModel) moveCursorUp(n int) {
	for n > 0 && m.cursor > 0 {
		m.cursor--
		if m.isNavigable(m.cursor) {
			n--
		}
	}
	// If loop ended on a non-navigable line, step forward to the first navigable one.
	for !m.isNavigable(m.cursor) && m.cursor < len(m.lineRefs)-1 {
		m.cursor++
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
	m.goToFirstNavigable()
}

func (m *diffViewModel) goToBottom() {
	max := len(m.lines) - m.viewHeight()
	if max < 0 {
		max = 0
	}
	m.offset = max
	m.goToLastNavigable()
}

func (m *diffViewModel) pageDown() {
	m.moveCursorDown(m.viewHeight())
}

func (m *diffViewModel) pageUp() {
	m.moveCursorUp(m.viewHeight())
}

func (m *diffViewModel) viewHeight() int {
	h := m.height - 1 // file header row
	if h < 1 {
		h = 1
	}
	return h
}

// clickToAbsIdx converts a panel-relative click Y (0 = file header row + 1)
// to an absolute index into lines[], accounting for any visible input box.
// Returns -1 if the click lands inside the input box itself.
func (m diffViewModel) clickToAbsIdx(clickY int) int {
	if !m.commentInputActive {
		return m.offset + clickY
	}
	codeAbove := m.cursor - m.offset + 1
	if clickY < codeAbove {
		return m.offset + clickY
	}
	if clickY < codeAbove+inputBoxHeight {
		return -1 // inside the input box
	}
	// Below the input box — map back to lines[], skipping the box rows.
	nextIdx := m.cursor + 1
	if m.isCommentDisplayLine(nextIdx) {
		nextIdx++ // this display line was skipped in the render
	}
	return nextIdx + (clickY - codeAbove - inputBoxHeight)
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

// linePrefix returns the 1-character cursor indicator for a display line.
func (m diffViewModel) linePrefix(absIdx int) string {
	if absIdx == m.cursor {
		return cursorStyle.Render("▶")
	}
	return " "
}

// inputBoxHeight is the number of rows the inline comment input box occupies.
const inputBoxHeight = 3 // border top + content + border bottom

func (m diffViewModel) render(focused bool) string {
	fileHeader := m.renderFileHeader()

	if len(m.lines) == 0 {
		content := lipgloss.NewStyle().Width(m.width).Height(m.height - 1).Render("")
		return fileHeader + "\n" + content
	}

	viewH := m.viewHeight()
	maxWidth := m.width - 1 // cursor prefix (1)
	if maxWidth < 1 {
		maxWidth = 1
	}

	renderLine := func(absIdx int) string {
		if ref := m.lineRefs[absIdx]; ref != nil && ref.isHunkHeader {
			return hunkHeaderStyle.Width(m.width).Render(" " + m.lines[absIdx])
		}
		line := m.lines[absIdx]
		if ansi.StringWidth(line) > maxWidth {
			line = ansi.Truncate(line, maxWidth, "")
		}
		return m.linePrefix(absIdx) + line
	}

	var renderedLines []string

	if m.commentInputActive {
		codeAbove := m.cursor - m.offset + 1
		if codeAbove < 0 {
			codeAbove = 0
		}
		end := m.offset + codeAbove
		if end > len(m.lines) {
			end = len(m.lines)
		}
		for absIdx := m.offset; absIdx < end; absIdx++ {
			renderedLines = append(renderedLines, renderLine(absIdx))
		}

		// Skip any existing comment display line for this code line —
		// the input box replaces it while editing.
		nextIdx := m.cursor + 1
		if m.isCommentDisplayLine(nextIdx) {
			nextIdx++
		}

		// Inline input box.
		inputWidth := m.width - 4
		if inputWidth < 10 {
			inputWidth = 10
		}
		m.textInput.Width = inputWidth - 4
		inputBox := commentInputStyle.Width(inputWidth).Render(m.textInput.View())
		renderedLines = append(renderedLines, inputBox)

		codeBelow := viewH - inputBoxHeight - codeAbove
		if codeBelow < 0 {
			codeBelow = 0
		}
		endAfter := nextIdx + codeBelow
		if endAfter > len(m.lines) {
			endAfter = len(m.lines)
		}
		for absIdx := nextIdx; absIdx < endAfter; absIdx++ {
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
	panel := lipgloss.NewStyle().Width(m.width).Height(m.height - 1).Render(content)
	return fileHeader + "\n" + panel
}

package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/justincampbell/revise/internal/git"
)

// lineRef tracks the source line metadata for a rendered display line.
// isCommentDisplay is true for comment annotation lines inserted below code lines.
// isContinuation is true for soft-wrap continuation rows that share their
// metadata with the parent navigable row above them.
// nil is used for non-content lines (file header, hunk header, blank separators).
type lineRef struct {
	newNum           int
	oldNum           int
	lineType         git.LineType
	isCommentDisplay bool
	isContinuation   bool
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
	file           *git.FileDiff
	lines          []string   // pre-rendered display lines
	lineRefs       []*lineRef // parallel to lines
	cursor         int        // absolute index into lines[]
	offset         int        // vertical scroll offset (top line index)
	hOffset        int        // horizontal scroll offset (columns clipped from the left)
	height         int
	width          int
	lastBuildWidth int // width used the last time buildLines ran; triggers wrap-aware rebuild on resize
	softWrap       bool
	comments       comments
	marks          marks

	commentInputActive bool
	fileCommentInput   bool // true when editing a file-level comment (appears before first hunk)
	textInput          textinput.Model
	fileReviewMode     bool // true when reviewing a file (suppresses git chrome in render)
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
	m.hOffset = 0
	m.buildLines()
	m.goToFirstNavigable()
}

func (m *diffViewModel) buildLines() {
	m.lines = nil
	m.lineRefs = nil
	m.lastBuildWidth = m.width

	if m.file == nil {
		m.lines = []string{"No file selected"}
		m.lineRefs = []*lineRef{nil}
		return
	}

	add := func(s string, ref *lineRef) {
		m.lines = append(m.lines, s)
		m.lineRefs = append(m.lineRefs, ref)
	}

	// Binary files: show a placeholder instead of diff content.
	if m.file.IsBinary {
		add("  Binary file — cannot display diff", nil)
		return
	}

	// Show file-level comment (lineNum == 0) at the top if one exists.
	fileKey := commentKey{file: m.file.Path, lineNum: 0}
	if text, ok := m.comments[fileKey]; ok {
		displayRef := &lineRef{isCommentDisplay: true}
		add(commentDisplayStyle.Render("  ◆ "+text), displayRef)
		add("", nil) // blank separator
	}

	p := paletteFor(activeTheme, activeIsDark)
	indentSize := detectIndentSize(m.file.Hunks)
	for _, hunk := range m.file.Hunks {
		if header := renderHunkHeader(hunk); header != "" {
			add(header, nil)
		}
		for _, line := range hunk.Lines {
			ref := &lineRef{
				newNum:   line.NewNum,
				oldNum:   line.OldNum,
				lineType: line.Type,
			}
			isMarked := m.marks[ref.commentKey(m.file.Path)]
			// width - 3 for border (2) + cursor prefix (1)
			fillWidth := m.width - 3
			if fillWidth < 1 {
				fillWidth = 1
			}
			rows := renderDiffLineRows(line, isMarked, fillWidth, m.file.Path, p, indentSize, m.softWrap)
			add(rows[0], ref)
			for _, contRow := range rows[1:] {
				contRef := &lineRef{
					newNum:         line.NewNum,
					oldNum:         line.OldNum,
					lineType:       line.Type,
					isContinuation: true,
				}
				add(contRow, contRef)
			}

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

// rebuildLinesPreservingCursor rebuilds display lines and restores the cursor
// to the same code line after the rebuild (handles index shifts from added/removed
// comment display lines).
func (m *diffViewModel) rebuildLinesPreservingCursor() {
	var saved *lineRef
	if m.cursor >= 0 && m.cursor < len(m.lineRefs) {
		ref := m.lineRefs[m.cursor]
		if ref != nil && !ref.isCommentDisplay {
			saved = ref
		}
	}

	m.buildLines()

	if saved == nil {
		return
	}
	for i, ref := range m.lineRefs {
		if ref != nil && !ref.isCommentDisplay && !ref.isContinuation &&
			ref.newNum == saved.newNum &&
			ref.oldNum == saved.oldNum &&
			ref.lineType == saved.lineType {
			m.cursor = i
			m.scrollCursorIntoView()
			return
		}
	}
}

// setSoftWrap toggles soft wrap and rebuilds display lines.
func (m *diffViewModel) setSoftWrap(on bool) {
	if m.softWrap == on {
		return
	}
	m.softWrap = on
	if on {
		m.hOffset = 0
	}
	m.rebuildLinesPreservingCursor()
}

// rebuildIfWidthChanged rebuilds lines when softWrap is on and the panel
// width has changed since the last build, since wrap output depends on width.
func (m *diffViewModel) rebuildIfWidthChanged() {
	if m.softWrap && m.lastBuildWidth != m.width {
		m.rebuildLinesPreservingCursor()
	}
}

// lastContinuationOf returns the index of the last continuation row of the
// logical line at idx (or idx itself when there are no continuations or idx
// is not navigable).
func (m diffViewModel) lastContinuationOf(idx int) int {
	if idx < 0 || idx >= len(m.lineRefs) {
		return idx
	}
	last := idx
	for last+1 < len(m.lineRefs) {
		next := m.lineRefs[last+1]
		if next == nil || !next.isContinuation {
			break
		}
		last++
	}
	return last
}

// isNavigable reports whether the line at idx can receive the cursor.
// Only code lines (non-nil, non-comment-display, non-continuation) are navigable.
func (m diffViewModel) isNavigable(idx int) bool {
	if idx < 0 || idx >= len(m.lineRefs) {
		return false
	}
	ref := m.lineRefs[idx]
	return ref != nil && !ref.isCommentDisplay && !ref.isContinuation
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
	m.scrollCursorIntoView()
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
	m.scrollCursorIntoView()
}

// scrollCursorIntoView adjusts offset so the cursor — and, in soft-wrap
// mode, all of its continuation rows — are visible. If the wrapped line is
// taller than the viewport, the cursor's first row is pinned to the top.
func (m *diffViewModel) scrollCursorIntoView() {
	viewH := m.viewHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
		return
	}
	lastRow := m.lastContinuationOf(m.cursor)
	if lastRow >= m.offset+viewH {
		m.offset = lastRow - viewH + 1
	}
	if m.offset > m.cursor {
		m.offset = m.cursor
	}
	if m.offset < 0 {
		m.offset = 0
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

// viewWidth returns the width available for a rendered line's content,
// excluding the panel border and cursor prefix column.
func (m *diffViewModel) viewWidth() int {
	w := m.width - 3 // border (2) + cursor prefix (1)
	if w < 1 {
		w = 1
	}
	return w
}

// maxHScroll returns the largest hOffset that keeps some content visible.
func (m *diffViewModel) maxHScroll() int {
	maxW := 0
	for _, line := range m.lines {
		if w := ansi.StringWidth(line); w > maxW {
			maxW = w
		}
	}
	max := maxW - m.viewWidth()
	if max < 0 {
		return 0
	}
	return max
}

func (m *diffViewModel) scrollRight(n int) {
	m.hOffset += n
	if max := m.maxHScroll(); m.hOffset > max {
		m.hOffset = max
	}
}

func (m *diffViewModel) scrollLeft(n int) {
	m.hOffset -= n
	if m.hOffset < 0 {
		m.hOffset = 0
	}
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

// hunkStarts returns the indices of the first navigable line in each hunk.
func (m *diffViewModel) hunkStarts() []int {
	var starts []int
	for i, ref := range m.lineRefs {
		if ref == nil || ref.isCommentDisplay {
			continue
		}
		// A navigable line is a hunk start if the previous non-comment line is nil (hunk header or blank separator).
		if i == 0 || m.lineRefs[i-1] == nil {
			starts = append(starts, i)
		}
	}
	return starts
}

// nextHunk moves the cursor to the first navigable line of the next hunk.
// If already in the last hunk, jumps to the last navigable line.
func (m *diffViewModel) nextHunk() {
	starts := m.hunkStarts()
	for _, idx := range starts {
		if idx > m.cursor {
			m.cursor = idx
			m.scrollCursorIntoView()
			return
		}
	}
	// No next hunk found — jump to last navigable line (past the last hunk).
	m.goToLastNavigable()
	m.scrollCursorIntoView()
}

// prevHunk moves the cursor to the first navigable line of the previous hunk.
// If the cursor is in the middle of a hunk (not on its first line), it moves
// to the start of the current hunk instead.
func (m *diffViewModel) prevHunk() {
	starts := m.hunkStarts()
	for i := len(starts) - 1; i >= 0; i-- {
		if starts[i] < m.cursor {
			m.cursor = starts[i]
			m.scrollCursorIntoView()
			return
		}
	}
}

func (m *diffViewModel) viewHeight() int {
	h := m.height
	if h < 1 {
		h = 1
	}
	return h
}

// clickToAbsIdx converts a panel-relative click Y (0 = top border row + 1)
// to an absolute index into lines[], accounting for any visible input box.
// Returns -1 if the click lands inside the input box itself.
func (m diffViewModel) clickToAbsIdx(clickY int) int {
	if !m.commentInputActive {
		return m.offset + clickY
	}
	cursorEnd := m.lastContinuationOf(m.cursor)
	codeAbove := cursorEnd - m.offset + 1
	if clickY < codeAbove {
		return m.offset + clickY
	}
	if clickY < codeAbove+inputBoxHeight {
		return -1 // inside the input box
	}
	// Below the input box — map back to lines[], skipping the box rows.
	nextIdx := cursorEnd + 1
	if m.isCommentDisplayLine(nextIdx) {
		nextIdx++ // this display line was skipped in the render
	}
	return nextIdx + (clickY - codeAbove - inputBoxHeight)
}

// gutterWidth is the visible column count for FormatGutter output (5-char
// number + 1 space). Continuation rows use a blank string of this width to
// keep wrapped content aligned under the original.
const gutterWidth = 6

// renderDiffLine returns one styled string for a diff line. Use
// renderDiffLineRows for soft-wrap aware multi-row output.
func renderDiffLine(l git.Line, marked bool, fillWidth int, filePath string, p themeColors, indentSize int) string {
	gutterStr, contentStr := renderDiffLineParts(l, marked, fillWidth, filePath, p, indentSize)
	return gutterStr + contentStr
}

// renderDiffLineParts builds the styled gutter and content separately so
// callers can wrap or otherwise combine them (e.g. continuation rows in
// soft-wrap mode reuse the gutter slot for blank padding).
func renderDiffLineParts(l git.Line, marked bool, fillWidth int, filePath string, p themeColors, indentSize int) (string, string) {
	gutter := git.FormatGutter(l)
	if marked {
		content := l.Content
		contentWidth := fillWidth - gutterWidth
		if contentWidth > 0 && len(content) < contentWidth {
			content += strings.Repeat(" ", contentWidth-len(content))
		}
		switch l.Type {
		case git.LineAdded:
			return markGutterAdded.Render(gutter), markAddedStyle.Render(content)
		case git.LineRemoved:
			return markGutterRemoved.Render(gutter), markRemovedStyle.Render(content)
		case git.LineContext:
			return markGutterContext.Render(gutter), markContextStyle.Render(content)
		}
		return "", l.Content
	}
	switch l.Type {
	case git.LineAdded:
		if highlighted, ok := highlightLine(l.Content, filePath, p.addedBg, indentSize); ok {
			return addedGutterStyle.Render(gutter), highlighted
		}
		return addedGutterStyle.Render(gutter), addedStyle.Render(addIndentGuides(l.Content, indentSize, p.addedBg))
	case git.LineRemoved:
		if highlighted, ok := highlightLine(l.Content, filePath, p.removedBg, indentSize); ok {
			return removedGutterStyle.Render(gutter), highlighted
		}
		return removedGutterStyle.Render(gutter), removedStyle.Render(addIndentGuides(l.Content, indentSize, p.removedBg))
	case git.LineContext:
		if highlighted, ok := highlightLine(l.Content, filePath, nil, indentSize); ok {
			return contextGutterStyle.Render(gutter), highlighted
		}
		return contextGutterStyle.Render(gutter), contextStyle.Render(addIndentGuides(l.Content, indentSize, nil))
	}
	return "", l.Content
}

// renderDiffLineRows returns one or more styled rows for a diff line. With
// softWrap=false or content that fits within fillWidth, it returns a single
// row (gutter + content). With wrap on, content is hard-wrapped at
// (fillWidth - gutterWidth) and continuation rows are prefixed with a
// blank-padded gutter so wrapped content stays aligned.
func renderDiffLineRows(l git.Line, marked bool, fillWidth int, filePath string, p themeColors, indentSize int, softWrap bool) []string {
	gutterStr, contentStr := renderDiffLineParts(l, marked, fillWidth, filePath, p, indentSize)
	if !softWrap {
		return []string{gutterStr + contentStr}
	}
	contentLimit := fillWidth - gutterWidth
	if contentLimit < 1 {
		return []string{gutterStr + contentStr}
	}
	if ansi.StringWidth(contentStr) <= contentLimit {
		return []string{gutterStr + contentStr}
	}
	wrapped := ansi.Hardwrap(contentStr, contentLimit, false)
	parts := strings.Split(wrapped, "\n")
	rows := make([]string, 0, len(parts))
	blankGutter := strings.Repeat(" ", gutterWidth)
	for i, part := range parts {
		if i == 0 {
			rows = append(rows, gutterStr+part)
		} else {
			rows = append(rows, blankGutter+part)
		}
	}
	return rows
}

func renderHunkHeader(h git.Hunk) string {
	header := git.HunkContextText(h.Header)
	label := git.HunkSourceLabel(h.Source)

	// No source and no header context — nothing to show (e.g. file review mode).
	if label == "" && header == "" {
		return ""
	}

	style := hunkStyle
	switch h.Source {
	case git.SourceBranch:
		style = hunkBranchStyle
	case git.SourceStaged:
		style = hunkStagedStyle
	case git.SourceUnstaged:
		style = hunkUnstagedStyle
	}

	tag := hunkSourceTagStyle.Render("[" + label + "]")
	if header == "" {
		return tag
	}
	if label == "" {
		return style.Render(header)
	}
	return tag + " " + style.Render(header)
}

// linePrefix returns the 1-character indicator for a display line — cursor
// (when focused), else a bookmark stripe for commented or marked lines, else
// a blank. Comments win over marks because they carry richer information;
// the mark's colored gutter background is still visible.
func (m diffViewModel) linePrefix(absIdx int, focused bool) string {
	if focused && m.file != nil && absIdx == m.cursor {
		return cursorStyle.Render("▶")
	}
	if m.lineCommented(absIdx) {
		return commentPrefixStyle.Render("▌")
	}
	if m.lineMarked(absIdx) {
		return markPrefixStyle.Render("▌")
	}
	return " "
}

// lineMarked reports whether the given display line corresponds to a marked
// source line.
func (m diffViewModel) lineMarked(absIdx int) bool {
	ref := m.codeLineRef(absIdx)
	if ref == nil {
		return false
	}
	return m.marks[ref.commentKey(m.file.Path)]
}

// lineCommented reports whether the given display line corresponds to a
// source line that has a saved comment.
func (m diffViewModel) lineCommented(absIdx int) bool {
	ref := m.codeLineRef(absIdx)
	if ref == nil {
		return false
	}
	_, ok := m.comments[ref.commentKey(m.file.Path)]
	return ok
}

// codeLineRef returns the lineRef for a real code line at absIdx, or nil
// if the index is out of range, points at a non-code row, or no file is open.
func (m diffViewModel) codeLineRef(absIdx int) *lineRef {
	if m.file == nil || !m.isNavigable(absIdx) {
		return nil
	}
	return m.lineRefs[absIdx]
}

// currentHunkIndex returns the index of the hunk containing the cursor line,
// or -1 if the cursor is not on a navigable line.
func (m *diffViewModel) currentHunkIndex() int {
	if m.file == nil || !m.isNavigable(m.cursor) {
		return -1
	}

	// Walk backwards from cursor to find which hunk contains this line.
	// Hunk boundaries are marked by nil lineRefs (hunk headers / blank separators).
	hunkIdx := -1
	for i := m.cursor; i >= 0; i-- {
		if m.lineRefs[i] == nil {
			// Count nil→navigable transitions to determine hunk index.
			break
		}
	}

	// Count hunk starts up to and including cursor position.
	starts := m.hunkStarts()
	for i, start := range starts {
		if start <= m.cursor {
			hunkIdx = i
		} else {
			break
		}
	}
	return hunkIdx
}

// inputBoxHeight is the number of rows the inline comment input box occupies.
const inputBoxHeight = 3 // border top + content + border bottom

func (m diffViewModel) render(focused bool, contextLines int, hideWhitespace bool, modeSlider ...string) string {
	viewH := m.viewHeight()
	maxWidth := m.viewWidth()
	// Clamp a stale offset (e.g. after a resize or a shorter refreshed diff)
	// so a scroll position from a wider state doesn't over-truncate now.
	hOffset := m.hOffset
	if m.softWrap {
		hOffset = 0
	} else if max := m.maxHScroll(); hOffset > max {
		hOffset = max
	}

	renderLine := func(absIdx int) string {
		line := m.lines[absIdx]
		if hOffset > 0 {
			line = ansi.TruncateLeft(line, hOffset, "")
		}
		if ansi.StringWidth(line) > maxWidth {
			line = ansi.Truncate(line, maxWidth, "")
		}
		return m.linePrefix(absIdx, focused) + line
	}

	var renderedLines []string
	if len(m.lines) == 0 {
		renderedLines = append(renderedLines, "No changes to display")
	}

	if m.commentInputActive && m.fileCommentInput && len(m.lines) > 0 {
		// File-level comment: input box appears before any code lines.
		inputWidth := m.width - 4
		if inputWidth < 10 {
			inputWidth = 10
		}
		m.textInput.Width = inputWidth - 4
		inputBox := commentInputStyle.Width(inputWidth).Render(m.textInput.View())
		renderedLines = append(renderedLines, inputBox)

		codeBelow := viewH - inputBoxHeight
		if codeBelow < 0 {
			codeBelow = 0
		}
		end := codeBelow
		if end > len(m.lines) {
			end = len(m.lines)
		}
		for absIdx := 0; absIdx < end; absIdx++ {
			renderedLines = append(renderedLines, renderLine(absIdx))
		}
	} else if m.commentInputActive && len(m.lines) > 0 {
		// In soft-wrap mode the cursor's logical line may span multiple
		// display rows; the input box appears below the last continuation.
		cursorEnd := m.lastContinuationOf(m.cursor)
		codeAbove := cursorEnd - m.offset + 1
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
		nextIdx := cursorEnd + 1
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
	} else if len(m.lines) > 0 {
		end := m.offset + viewH
		if end > len(m.lines) {
			end = len(m.lines)
		}
		for absIdx := m.offset; absIdx < end; absIdx++ {
			renderedLines = append(renderedLines, renderLine(absIdx))
		}
	}

	added, removed := 0, 0
	if m.file != nil {
		added, removed = fileTotals(*m.file)
	}
	content := strings.Join(renderedLines, "\n")

	style := panelBorder
	if focused {
		style = focusedBorder
	}
	rendered := style.Width(m.width).Height(m.height).MaxHeight(m.height + 2).Render(content)

	if m.fileReviewMode {
		// File review mode: centered filename in top border, clean bottom border.
		slider := ""
		if len(modeSlider) > 0 {
			slider = modeSlider[0]
		}
		if slider != "" {
			rendered = setBorderTitleCentered(rendered, fileStyle.Render(" "+slider+" "), focused)
		}
		if m.softWrap {
			rendered = setBorderBottomRight(rendered, modeActiveStyle.Render(" Wrap "), focused)
		}
		return rendered
	}

	title := ""
	if m.file != nil {
		title = " " + m.file.Path + " "
		if m.file.OldPath != "" {
			title = " " + m.file.OldPath + " → " + m.file.Path + " "
		}
	}
	slider := ""
	if len(modeSlider) > 0 {
		slider = modeSlider[0]
	}
	if slider != "" && title != "" {
		rendered = setBorderTitleLeftAndCenter(rendered, title, slider, focused)
	} else if slider != "" {
		rendered = setBorderTitleCentered(rendered, slider, focused)
	} else if title != "" {
		rendered = setBorderTitle(rendered, title, focused)
	}
	rendered = setBorderBottomCounts(rendered, added, removed, focused)
	ctxText := fmt.Sprintf(" Context: %d ", contextLines)
	ctxLabel := statusBarStyle.Render(ctxText)
	if contextLines != git.DefaultContextLines {
		ctxLabel = modeActiveStyle.Render(ctxText)
	}
	rendered = setBorderBottomLeft(rendered, ctxLabel, focused)
	var rightLabel string
	if hideWhitespace {
		rightLabel = modeActiveStyle.Render(" Whitespace hidden ")
	} else {
		rightLabel = statusBarStyle.Render(" Whitespace ")
	}
	if m.softWrap {
		rightLabel = modeActiveStyle.Render(" Wrap ") + rightLabel
	}
	rendered = setBorderBottomRight(rendered, rightLabel, focused)
	return rendered
}

// setBorderTitle overlays a styled title onto the top border of a lipgloss-rendered box.
// Lipgloss v1.1.0 doesn't support border titles natively, so we reconstruct
// the top border line: ╭ + title + remaining ─ chars + ╮.
func setBorderTitle(rendered, title string, focused bool) string {
	nl := strings.IndexByte(rendered, '\n')
	if nl < 0 {
		return rendered
	}
	topLine := rendered[:nl]
	rest := rendered[nl:]

	borderColor := colorBorder
	if focused {
		borderColor = colorCyan
	}
	bc := lipgloss.NewStyle().Foreground(borderColor)

	totalWidth := ansi.StringWidth(topLine)
	titleRendered := fileStyle.Render(title)
	titleWidth := ansi.StringWidth(titleRendered)

	// Need at least room for ╭ + title + ╮
	if titleWidth+2 > totalWidth {
		return rendered
	}

	fillWidth := totalWidth - 1 - titleWidth - 1 // minus ╭ and ╮
	if fillWidth < 0 {
		fillWidth = 0
	}

	newTop := bc.Render("╭") + titleRendered + bc.Render(strings.Repeat("─", fillWidth)) + bc.Render("╮")
	return newTop + rest
}

// setBorderTitleLeftAndCenter builds a top border with a left-aligned title and a centered title.
// If they would overlap, only the left title is shown.
func setBorderTitleLeftAndCenter(rendered, leftTitle, centerTitle string, focused bool) string {
	nl := strings.IndexByte(rendered, '\n')
	if nl < 0 {
		return rendered
	}
	topLine := rendered[:nl]
	rest := rendered[nl:]

	borderColor := colorBorder
	if focused {
		borderColor = colorCyan
	}
	bc := lipgloss.NewStyle().Foreground(borderColor)

	totalWidth := ansi.StringWidth(topLine)
	leftRendered := fileStyle.Render(leftTitle)
	leftWidth := ansi.StringWidth(leftRendered)
	centerWidth := ansi.StringWidth(centerTitle)

	// Center position calculation
	innerWidth := totalWidth - 2 // minus ╭ and ╮
	centerStart := (innerWidth - centerWidth) / 2
	centerEnd := centerStart + centerWidth

	// Left title occupies positions 0..leftWidth-1 (after ╭)
	// If left title would overlap center, fall back to left-only
	if leftWidth >= centerStart || centerEnd > innerWidth {
		return setBorderTitle(rendered, leftTitle, focused)
	}

	// Build: ╭ + leftTitle + fill + centerTitle + fill + ╮
	gapBeforeCenter := centerStart - leftWidth
	gapAfterCenter := innerWidth - centerEnd

	newTop := bc.Render("╭") +
		leftRendered +
		bc.Render(strings.Repeat("─", gapBeforeCenter)) +
		centerTitle +
		bc.Render(strings.Repeat("─", gapAfterCenter)) +
		bc.Render("╮")
	return newTop + rest
}

// setBorderTitleCentered overlays a centered pre-rendered title onto the top border.
func setBorderTitleCentered(rendered, title string, focused bool) string {
	nl := strings.IndexByte(rendered, '\n')
	if nl < 0 {
		return rendered
	}
	topLine := rendered[:nl]
	rest := rendered[nl:]

	borderColor := colorBorder
	if focused {
		borderColor = colorCyan
	}
	bc := lipgloss.NewStyle().Foreground(borderColor)

	totalWidth := ansi.StringWidth(topLine)
	titleWidth := ansi.StringWidth(title)

	if titleWidth+2 > totalWidth {
		return rendered
	}

	fillWidth := totalWidth - 1 - titleWidth - 1 // minus ╭ and ╮
	leftFill := fillWidth / 2
	rightFill := fillWidth - leftFill

	newTop := bc.Render("╭") + bc.Render(strings.Repeat("─", leftFill)) + title + bc.Render(strings.Repeat("─", rightFill)) + bc.Render("╮")
	return newTop + rest
}

// setBorderBottomLeft overlays a left-aligned label onto an existing bottom border,
// preserving content to the right (e.g. centered counts).
func setBorderBottomLeft(rendered, title string, focused bool) string {
	lastNl := strings.LastIndexByte(rendered, '\n')
	if lastNl < 0 {
		return rendered
	}
	rest := rendered[:lastNl+1]
	bottomLine := rendered[lastNl+1:]

	borderColor := colorBorder
	if focused {
		borderColor = colorCyan
	}
	bc := lipgloss.NewStyle().Foreground(borderColor)

	totalWidth := ansi.StringWidth(bottomLine)
	titleWidth := ansi.StringWidth(title)

	if titleWidth+2 > totalWidth {
		return rendered
	}

	// Rebuild: ╰ + title + remainder of existing bottom line from that point on
	rightStart := 1 + titleWidth // skip ╰ corner + title width
	right := ansi.TruncateLeft(bottomLine, rightStart, "")
	// If truncation left a gap, pad with border chars
	rightWidth := ansi.StringWidth(right)
	if rightWidth < totalWidth-rightStart {
		right += bc.Render(strings.Repeat("─", totalWidth-rightStart-rightWidth))
	}

	newBottom := bc.Render("╰") + title + right
	return rest + newBottom
}

// setBorderBottomRight overlays a right-aligned label onto an existing bottom border,
// preserving content to the left (e.g. centered counts, left-aligned context).
func setBorderBottomRight(rendered, title string, focused bool) string {
	lastNl := strings.LastIndexByte(rendered, '\n')
	if lastNl < 0 {
		return rendered
	}
	rest := rendered[:lastNl+1]
	bottomLine := rendered[lastNl+1:]

	borderColor := colorBorder
	if focused {
		borderColor = colorCyan
	}
	bc := lipgloss.NewStyle().Foreground(borderColor)

	totalWidth := ansi.StringWidth(bottomLine)
	titleWidth := ansi.StringWidth(title)

	if titleWidth+2 > totalWidth {
		return rendered
	}

	// Truncate existing bottom to make room for title + ╯
	leftWidth := totalWidth - titleWidth - 1 // minus ╯
	left := ansi.Truncate(bottomLine, leftWidth, "")
	// Pad if truncation came up short
	for ansi.StringWidth(left) < leftWidth {
		left += bc.Render("─")
	}

	newBottom := left + title + bc.Render("╯")
	return rest + newBottom
}

// setBorderBottomTitle overlays a centered title onto the bottom border line.
func setBorderBottomTitle(rendered, title string, focused bool) string {
	lastNl := strings.LastIndexByte(rendered, '\n')
	if lastNl < 0 {
		return rendered
	}
	rest := rendered[:lastNl+1]
	bottomLine := rendered[lastNl+1:]

	borderColor := colorBorder
	if focused {
		borderColor = colorCyan
	}
	bc := lipgloss.NewStyle().Foreground(borderColor)

	totalWidth := ansi.StringWidth(bottomLine)
	titleWidth := ansi.StringWidth(title)

	if titleWidth+2 > totalWidth {
		return rendered
	}

	fillWidth := totalWidth - 1 - titleWidth - 1 // minus ╰ and ╯
	leftFill := fillWidth / 2
	rightFill := fillWidth - leftFill

	newBottom := bc.Render("╰") + bc.Render(strings.Repeat("─", leftFill)) + title + bc.Render(strings.Repeat("─", rightFill)) + bc.Render("╯")
	return rest + newBottom
}

// setBorderBottomCounts renders " +A/-R " on the bottom border with the slash
// anchored to the pane center so it doesn't shift as digit widths change.
func setBorderBottomCounts(rendered string, added, removed int, focused bool) string {
	lastNl := strings.LastIndexByte(rendered, '\n')
	if lastNl < 0 {
		return rendered
	}
	rest := rendered[:lastNl+1]
	bottomLine := rendered[lastNl+1:]

	borderColor := colorBorder
	if focused {
		borderColor = colorCyan
	}
	bc := lipgloss.NewStyle().Foreground(borderColor)

	totalWidth := ansi.StringWidth(bottomLine)
	contentWidth := totalWidth - 2 // exclude corners
	if contentWidth < 1 {
		return rendered
	}

	left := " " + statusAdded.Render(fmt.Sprintf("+%d", added))
	slash := statusBarStyle.Render("/")
	right := statusDeleted.Render(fmt.Sprintf("-%d", removed)) + " "

	leftWidth := ansi.StringWidth(left)
	rightWidth := ansi.StringWidth(right)
	center := contentWidth / 2 // slash column in inner area

	leftFill := center - leftWidth
	rightFill := contentWidth - center - 1 - rightWidth // minus slash
	if leftFill < 0 || rightFill < 0 {
		// Fallback to centered title for very narrow panes.
		return setBorderBottomTitle(rendered, renderPaneChangeSummary(added, removed), focused)
	}

	newBottom := bc.Render("╰") +
		bc.Render(strings.Repeat("─", leftFill)) +
		left +
		slash +
		right +
		bc.Render(strings.Repeat("─", rightFill)) +
		bc.Render("╯")
	return rest + newBottom
}

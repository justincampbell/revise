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
// nil is used for non-content lines (file header, hunk header, blank separators).
type lineRef struct {
	newNum           int
	oldNum           int
	lineType         git.LineType
	isCommentDisplay bool
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
		m.lines = []string{"No file selected"}
		m.lineRefs = []*lineRef{nil}
		return
	}

	add := func(s string, ref *lineRef) {
		m.lines = append(m.lines, s)
		m.lineRefs = append(m.lineRefs, ref)
	}

	// Show file-level comment (lineNum == 0) at the top if one exists.
	fileKey := commentKey{file: m.file.Path, lineNum: 0}
	if text, ok := m.comments[fileKey]; ok {
		displayRef := &lineRef{isCommentDisplay: true}
		add(commentDisplayStyle.Render("  ◆ "+text), displayRef)
		add("", nil) // blank separator
	}

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
		if ref != nil && !ref.isCommentDisplay &&
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
// Only code lines (non-nil, non-comment-display) are navigable.
func (m *diffViewModel) isNavigable(idx int) bool {
	if idx < 0 || idx >= len(m.lineRefs) {
		return false
	}
	ref := m.lineRefs[idx]
	return ref != nil && !ref.isCommentDisplay
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
			viewH := m.viewHeight()
			if m.cursor >= m.offset+viewH {
				m.offset = m.cursor - viewH + 1
			}
			return
		}
	}
	// No next hunk found — jump to last navigable line (past the last hunk).
	m.goToLastNavigable()
	viewH := m.viewHeight()
	if m.cursor >= m.offset+viewH {
		m.offset = m.cursor - viewH + 1
	}
}

// prevHunk moves the cursor to the first navigable line of the previous hunk.
// If the cursor is in the middle of a hunk (not on its first line), it moves
// to the start of the current hunk instead.
func (m *diffViewModel) prevHunk() {
	starts := m.hunkStarts()
	for i := len(starts) - 1; i >= 0; i-- {
		if starts[i] < m.cursor {
			m.cursor = starts[i]
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
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
	n := l.NewNum
	if l.Type == git.LineRemoved {
		n = l.OldNum
	}
	switch l.Type {
	case git.LineAdded, git.LineRemoved, git.LineContext:
		return fmt.Sprintf("%5d ", n)
	}
	return "      "
}

func renderHunkHeader(h git.Hunk) string {
	header := hunkContext(h.Header)

	style := hunkStyle
	var label string
	switch h.Source {
	case git.SourceBranch:
		style = hunkBranchStyle
		label = "branch"
	case git.SourceStaged:
		style = hunkStagedStyle
		label = "staged"
	case git.SourceUnstaged:
		style = hunkUnstagedStyle
		label = "unstaged"
	}

	tag := hunkSourceTagStyle.Render("[" + label + "]")
	if header == "" {
		return tag
	}
	return tag + " " + style.Render(header)
}

func hunkContext(header string) string {
	parts := strings.SplitN(header, "@@", 3)
	if len(parts) < 3 {
		return strings.TrimSpace(header)
	}
	return strings.TrimSpace(parts[2])
}

// linePrefix returns the 1-character cursor indicator for a display line.
// The cursor is only shown when the diff pane is focused.
func (m diffViewModel) linePrefix(absIdx int, focused bool) string {
	if focused && m.file != nil && absIdx == m.cursor {
		return cursorStyle.Render("▶")
	}
	return " "
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
	maxWidth := m.width - 3 // panel border (2) + cursor prefix (1)
	if maxWidth < 1 {
		maxWidth = 1
	}

	renderLine := func(absIdx int) string {
		line := m.lines[absIdx]
		if ansi.StringWidth(line) > maxWidth {
			line = ansi.Truncate(line, maxWidth, "")
		}
		return m.linePrefix(absIdx, focused) + line
	}

	var renderedLines []string
	if len(m.lines) == 0 {
		renderedLines = append(renderedLines, "No changes to display")
	}

	if m.commentInputActive && len(m.lines) > 0 {
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
	if hideWhitespace {
		wsLabel := modeActiveStyle.Render(" Whitespace hidden ")
		rendered = setBorderBottomRight(rendered, wsLabel, focused)
	} else {
		wsLabel := statusBarStyle.Render(" Whitespace ")
		rendered = setBorderBottomRight(rendered, wsLabel, focused)
	}
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

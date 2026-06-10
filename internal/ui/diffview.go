package ui

import (
	"fmt"
	"math"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"charm.land/bubbles/v2/textinput"
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
	isBlank          bool   // source line is empty/whitespace-only (paragraph boundary in file review)
	content          string // plain source text, used for incremental search matching
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
	offset   int        // vertical scroll offset (top line index)
	hOffset  int        // horizontal scroll offset (columns clipped from the left)
	height   int
	width    int
	comments comments
	marks    marks

	commentInputActive bool
	fileCommentInput   bool // true when editing a file-level comment (appears before first hunk)
	textInput          textinput.Model
	fileReviewMode     bool // true when reviewing a file (suppresses git chrome in render)
	wrapEnabled        bool // true when soft line wrap is on (long lines wrap to multiple rows)

	// Incremental search (#72). searchQuery drives both highlighting (in
	// buildLines/renderDiffLine) and match navigation. searchMatches holds
	// the navigable line indices that contain the query; searchIdx is the
	// position within searchMatches of the "current" match (-1 when none).
	searchInput   textinput.Model
	searchQuery   string
	searchMatches []int
	searchIdx     int
	searchOrigin  int // cursor when search started, so typing jumps to the nearest match forward
}

// gutterWidth returns the leading column width occupied by the line-number
// gutter for the line at idx. Code lines have a 6-column gutter (matching the
// gutter styles' Width(6)); non-code rows (hunk headers, comment displays,
// blank separators) have none. Used to indent wrapped continuation rows so
// content stays aligned under the first row.
const gutterWidth = 6

func (m diffViewModel) lineGutterWidth(idx int) int {
	if m.isNavigable(idx) {
		return gutterWidth
	}
	return 0
}

// displayRows returns the rendered display rows for the logical line at idx,
// given the available content width and horizontal scroll offset.
//
// When wrap is off, a logical line is always a single display row (horizontally
// scrolled and clipped). When wrap is on, a line wider than avail is soft-wrapped
// onto multiple rows; continuation rows are indented past the gutter so their
// content aligns under the first row's content.
func (m diffViewModel) displayRows(idx, avail, hOffset int) []string {
	line := m.lines[idx]
	if !m.wrapEnabled {
		if hOffset > 0 {
			line = ansi.TruncateLeft(line, hOffset, "")
		}
		if ansi.StringWidth(line) > avail {
			line = ansi.Truncate(line, avail, "")
		}
		return []string{line}
	}
	if avail < 1 || ansi.StringWidth(line) <= avail {
		return []string{line}
	}
	gw := m.lineGutterWidth(idx)
	contentAvail := avail - gw
	if contentAvail < 1 {
		contentAvail = 1
	}
	gutter := ansi.Truncate(line, gw, "")
	content := ansi.TruncateLeft(line, gw, "")
	segs := strings.Split(ansi.Wrap(content, contentAvail, ""), "\n")
	indent := strings.Repeat(" ", gw)
	rows := make([]string, len(segs))
	for i, s := range segs {
		if i == 0 {
			rows[i] = gutter + s
		} else {
			rows[i] = indent + s
		}
	}
	return rows
}

// lineRows returns how many display rows the logical line at idx occupies.
func (m diffViewModel) lineRows(idx, avail int) int {
	if idx < 0 || idx >= len(m.lines) {
		return 1
	}
	return len(m.displayRows(idx, avail, 0))
}

// rowSpan returns the total display rows occupied by logical lines [a, b].
func (m diffViewModel) rowSpan(a, b, avail int) int {
	rows := 0
	for i := a; i <= b && i < len(m.lines); i++ {
		rows += m.lineRows(i, avail)
	}
	return rows
}

// lineAtRow walks forward from logical line start by targetRow display rows and
// returns the logical line index landed on. Returns len(lines) when the target
// is past the end (callers bounds-check).
func (m diffViewModel) lineAtRow(start, targetRow, avail int) int {
	if !m.wrapEnabled {
		return start + targetRow
	}
	row := 0
	for i := start; i < len(m.lines); i++ {
		r := m.lineRows(i, avail)
		if targetRow < row+r {
			return i
		}
		row += r
	}
	return len(m.lines)
}

// ensureCursorVisible adjusts offset so the cursor line (including all of its
// wrapped display rows) is visible. It scrolls up when the cursor is above the
// viewport and down when the cursor's rows fall below it, but never scrolls
// further than necessary. When wrap is off this reduces to the classic
// offset = cursor - viewHeight + 1 clamp.
func (m *diffViewModel) ensureCursorVisible() {
	if m.cursor < m.offset {
		m.offset = m.cursor
		return
	}
	viewH := m.viewHeight()
	avail := m.viewWidth()
	rows := 0
	top := m.cursor
	for top >= 0 {
		r := m.lineRows(top, avail)
		if rows+r > viewH {
			break
		}
		rows += r
		top--
	}
	minTop := top + 1
	if minTop < 0 {
		minTop = 0
	}
	if m.offset < minTop {
		m.offset = minTop
	}
}

// bottomOffset returns the largest offset that keeps content filling the
// viewport from the last line upward — the scroll position for "bottom".
func (m *diffViewModel) bottomOffset() int {
	viewH := m.viewHeight()
	avail := m.viewWidth()
	rows := 0
	top := len(m.lines) - 1
	for top >= 0 {
		r := m.lineRows(top, avail)
		if rows+r > viewH {
			break
		}
		rows += r
		top--
	}
	off := top + 1
	if off < 0 {
		off = 0
	}
	return off
}

// lastVisibleLine returns the index of the last logical line that fits in the
// viewport starting at the current offset.
func (m *diffViewModel) lastVisibleLine() int {
	viewH := m.viewHeight()
	avail := m.viewWidth()
	rows := 0
	i := m.offset
	for i < len(m.lines) {
		r := m.lineRows(i, avail)
		if rows+r > viewH {
			if i == m.offset {
				return i // a single line taller than the viewport
			}
			break
		}
		rows += r
		i++
	}
	last := i - 1
	if last < 0 {
		last = 0
	}
	return last
}

// scrollForCommentInput scrolls offset down (if needed) so the inline comment
// input box and at least one following code row fit below the cursor within the
// viewport. It accounts for wrapped display rows, so a tall wrapped cursor line
// still leaves room for the box.
func (m *diffViewModel) scrollForCommentInput() {
	viewH := m.viewHeight()
	avail := m.viewWidth()
	needed := inputBoxHeight + 1 // box rows + at least one code row below
	for m.offset < m.cursor && m.rowSpan(m.offset, m.cursor, avail)+needed > viewH {
		m.offset++
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// toggleWrap flips soft wrap on/off and keeps the cursor visible. Horizontal
// scroll is meaningless with wrap on, so it's reset when enabling.
func (m *diffViewModel) toggleWrap() {
	m.wrapEnabled = !m.wrapEnabled
	if m.wrapEnabled {
		m.hOffset = 0
	}
	m.ensureCursorVisible()
}

func newDiffViewModel() diffViewModel {
	ti := textinput.New()
	ti.Placeholder = "Add a comment…"
	ti.CharLimit = 500

	si := textinput.New()
	si.Prompt = "" // the "/" prompt is rendered by the status bar
	si.Placeholder = ""
	si.CharLimit = 200

	return diffViewModel{
		comments:    make(comments),
		textInput:   ti,
		searchInput: si,
		searchIdx:   -1,
	}
}

func (m *diffViewModel) setFile(f *git.FileDiff) {
	m.file = f
	m.cursor = 0
	m.offset = 0
	m.hOffset = 0
	// Search is scoped to the current file; switching files clears it.
	m.searchQuery = ""
	m.searchMatches = nil
	m.searchIdx = -1
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
	isMarkdown := isMarkdownFile(m.file.Path)
	for _, hunk := range m.file.Hunks {
		if header := renderHunkHeader(hunk); header != "" {
			add(header, nil)
		}
		// Track fenced-code-block state within the hunk so code is highlighted
		// with its declared language (e.g. ```go). Reset per hunk: a diff's
		// hidden gaps make cross-hunk fence state unreliable.
		inCodeBlock := false
		codeLang := ""
		for _, line := range hunk.Lines {
			ref := &lineRef{
				newNum:   line.NewNum,
				oldNum:   line.OldNum,
				lineType: line.Type,
				isBlank:  strings.TrimSpace(line.Content) == "",
				content:  line.Content,
			}
			isMarked := m.marks[ref.commentKey(m.file.Path)]
			// width - 3 for border (2) + cursor prefix (1)
			fillWidth := m.width - 3
			if fillWidth < 1 {
				fillWidth = 1
			}
			// Resolve the language for lines inside a fenced code block; the
			// fence delimiters themselves render as plain Markdown.
			langOverride := ""
			if isMarkdown {
				if fence, lang := codeFenceLang(line.Content); fence {
					if inCodeBlock {
						inCodeBlock, codeLang = false, ""
					} else {
						inCodeBlock, codeLang = true, lang
					}
				} else if inCodeBlock {
					langOverride = codeLang
				}
			}
			add(renderDiffLine(line, isMarked, fillWidth, m.file.Path, p, indentSize, m.searchQuery, langOverride), ref)

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
			m.ensureCursorVisible()
			return
		}
	}
}

// isNavigable reports whether the line at idx can receive the cursor.
// Only code lines (non-nil, non-comment-display) are navigable.
func (m diffViewModel) isNavigable(idx int) bool {
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
	m.ensureCursorVisible()
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
	m.ensureCursorVisible()
}

func (m *diffViewModel) clampCursorToView() {
	if m.cursor < m.offset {
		m.cursor = m.offset
	}
	if last := m.lastVisibleLine(); m.cursor > last {
		m.cursor = last
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
	if max := m.bottomOffset(); m.offset > max {
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
	m.offset = m.bottomOffset()
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

// isBlankLine reports whether the line at idx is a navigable, blank source line
// — i.e. a Vim-style paragraph boundary that `}`/`{` jump between.
func (m *diffViewModel) isBlankLine(idx int) bool {
	if idx < 0 || idx >= len(m.lineRefs) {
		return false
	}
	ref := m.lineRefs[idx]
	return ref != nil && !ref.isCommentDisplay && ref.isBlank
}

// nextParagraph moves the cursor to the next blank line below it (Vim `}`).
// If the cursor is already on a blank line, it first skips the contiguous
// blank run, then the following paragraph, landing on the next blank line.
// With no blank line below, it jumps to the last navigable line.
func (m *diffViewModel) nextParagraph() {
	i := m.cursor
	if m.isBlankLine(i) {
		for i < len(m.lines) && m.isBlankLine(i) {
			i++
		}
	}
	for i < len(m.lines) && !m.isBlankLine(i) {
		i++
	}
	if i < len(m.lines) {
		m.cursor = i
	} else {
		m.goToLastNavigable()
	}
	m.ensureCursorVisible()
}

// prevParagraph moves the cursor to the previous blank line above it (Vim `{`).
// Mirror of nextParagraph. With no blank line above, it jumps to the top.
func (m *diffViewModel) prevParagraph() {
	i := m.cursor
	if m.isBlankLine(i) {
		for i >= 0 && m.isBlankLine(i) {
			i--
		}
	}
	for i >= 0 && !m.isBlankLine(i) {
		i--
	}
	if i >= 0 {
		m.cursor = i
	} else {
		// No blank line above — jump to the first navigable line (top).
		m.cursor = 0
		m.goToFirstNavigable()
	}
	m.ensureCursorVisible()
}

func (m *diffViewModel) viewHeight() int {
	h := m.height
	if h < 1 {
		h = 1
	}
	return h
}

// scrollMetrics returns the total display rows of all content, the display rows
// above the current viewport top (offset), and the viewport height in rows.
// The scrollbar thumb and the scroll-position label both derive from these so
// they always agree. Row counts (not logical-line counts) make the math correct
// when soft wrap is on.
func (m diffViewModel) scrollMetrics() (totalRows, rowsAbove, viewH int) {
	viewH = m.viewHeight()
	if len(m.lines) == 0 {
		return 0, 0, viewH
	}
	avail := m.viewWidth()
	totalRows = m.rowSpan(0, len(m.lines)-1, avail)
	if m.offset > 0 {
		rowsAbove = m.rowSpan(0, m.offset-1, avail)
	}
	return totalRows, rowsAbove, viewH
}

// scrollbarThumb returns the thumb's top row and size within the viewport track
// (viewH rows) and whether to draw it. show is false when all content fits or
// the viewport is too short to render a meaningful thumb.
func (m diffViewModel) scrollbarThumb() (top, size int, show bool) {
	totalRows, rowsAbove, viewH := m.scrollMetrics()
	if viewH < 2 || totalRows <= viewH {
		return 0, 0, false
	}
	size = int(math.Round(float64(viewH) * float64(viewH) / float64(totalRows)))
	if size < 1 {
		size = 1
	}
	if size > viewH-1 {
		size = viewH - 1 // leave at least one row of travel so the thumb can move
	}
	maxAbove := totalRows - viewH
	maxTop := viewH - size
	frac := 0.0
	if maxAbove > 0 {
		frac = float64(rowsAbove) / float64(maxAbove)
	}
	frac = math.Min(1, math.Max(0, frac))
	top = int(math.Round(float64(maxTop) * frac))
	if top < 0 {
		top = 0
	}
	if top > maxTop {
		top = maxTop
	}
	return top, size, true
}

// overlayScrollbar draws the scrollbar thumb onto the right border of an
// already-rendered, bordered box by replacing the rightmost column of the
// thumb's track rows with a solid block. A full block (rather than a heavy line
// like ┃, which some fonts render indistinguishably from the │ track) makes the
// thumb stand out, and it's colored cyan when the pane is focused. No-op when
// all content fits. Interior rows are lines[1 .. len-2]; track row t → lines[t+1].
func (m diffViewModel) overlayScrollbar(rendered string, focused bool) string {
	top, size, show := m.scrollbarThumb()
	if !show || size <= 0 {
		return rendered
	}
	lines := strings.Split(rendered, "\n")
	if len(lines) < 3 {
		return rendered // need a top border, ≥1 interior row, and a bottom border
	}
	interior := len(lines) - 2

	thumbColor := colorScrollbarThumb
	if focused {
		thumbColor = colorCyan
	}
	thumb := lipgloss.NewStyle().Foreground(thumbColor).Bold(true).Render("█")

	for t := top; t < top+size && t < interior; t++ {
		rowIdx := t + 1
		line := lines[rowIdx]
		w := ansi.StringWidth(line)
		if w < 1 {
			continue
		}
		lines[rowIdx] = ansi.Truncate(line, w-1, "") + thumb
	}
	return strings.Join(lines, "\n")
}

// matchRanges returns the rune-index ranges of every case-insensitive
// occurrence of query within content. Ranges are [start, end) in runes so
// callers can slice []rune(content) directly. Returns nil for an empty query
// or no matches.
func matchRanges(content, query string) [][2]int {
	q := []rune(query)
	if len(q) == 0 {
		return nil
	}
	c := []rune(content)
	lc := make([]rune, len(c))
	for i, r := range c {
		lc[i] = unicode.ToLower(r)
	}
	lq := make([]rune, len(q))
	for i, r := range q {
		lq[i] = unicode.ToLower(r)
	}

	var ranges [][2]int
	for i := 0; i+len(lq) <= len(lc); {
		matched := true
		for j := range lq {
			if lc[i+j] != lq[j] {
				matched = false
				break
			}
		}
		if matched {
			ranges = append(ranges, [2]int{i, i + len(lq)})
			i += len(lq)
		} else {
			i++
		}
	}
	return ranges
}

// matchColumnRanges converts the rune-index match ranges of query within
// content into visual-column ranges (accounting for wide runes), so callers
// can slice an already-styled string by column.
func matchColumnRanges(content, query string) [][2]int {
	rr := matchRanges(content, query)
	if rr == nil {
		return nil
	}
	runes := []rune(content)
	cols := make([][2]int, len(rr))
	for i, r := range rr {
		cols[i] = [2]int{
			ansi.StringWidth(string(runes[:r[0]])),
			ansi.StringWidth(string(runes[:r[1]])),
		}
	}
	return cols
}

// overlaySearchHighlight re-styles the given visual-column ranges of an
// already-styled string with searchMatchStyle, leaving the surrounding styling
// (syntax highlight, diff background) intact. Column ranges are [start, end)
// and must be ascending and non-overlapping.
func overlaySearchHighlight(styled string, colRanges [][2]int) string {
	var b strings.Builder
	prev := 0
	for _, r := range colRanges {
		cs, ce := r[0], r[1]
		if cs < prev {
			cs = prev
		}
		if ce <= cs {
			continue
		}
		if cs > prev {
			b.WriteString(clipCols(styled, prev, cs))
		}
		// Strip the matched slice's own styling, then apply the match style.
		b.WriteString(searchMatchStyle.Render(ansi.Strip(clipCols(styled, cs, ce))))
		prev = ce
	}
	b.WriteString(ansi.TruncateLeft(styled, prev, ""))
	return b.String()
}

// clipCols returns the visual columns [from, to) of a styled string, preserving
// ANSI styling at the cut points.
func clipCols(styled string, from, to int) string {
	if from > 0 {
		styled = ansi.TruncateLeft(styled, from, "")
	}
	return ansi.Truncate(styled, to-from, "")
}

// lineContainsQuery reports whether content contains query (case-insensitive).
func lineContainsQuery(content, query string) bool {
	if query == "" {
		return false
	}
	return strings.Contains(strings.ToLower(content), strings.ToLower(query))
}

// computeSearchMatches rebuilds searchMatches from the current searchQuery,
// scanning navigable code lines. searchIdx is reset to -1 when there are no
// matches and clamped into range otherwise.
func (m *diffViewModel) computeSearchMatches() {
	m.searchMatches = nil
	if m.searchQuery == "" {
		m.searchIdx = -1
		return
	}
	for i := range m.lineRefs {
		if !m.isNavigable(i) {
			continue
		}
		if lineContainsQuery(m.lineRefs[i].content, m.searchQuery) {
			m.searchMatches = append(m.searchMatches, i)
		}
	}
	if len(m.searchMatches) == 0 {
		m.searchIdx = -1
	} else if m.searchIdx < 0 || m.searchIdx >= len(m.searchMatches) {
		m.searchIdx = 0
	}
}

// startSearch begins an incremental search. searchOrigin records the cursor so
// typing jumps to the nearest match at or after it.
func (m *diffViewModel) startSearch() {
	m.searchOrigin = m.cursor
	m.searchQuery = ""
	m.searchMatches = nil
	m.searchIdx = -1
	m.searchInput.SetValue("")
	m.searchInput.Focus()
}

// setSearch updates the active query (incremental: called on each keystroke),
// recomputes highlights and matches, and moves the cursor to the first match at
// or after searchOrigin (wrapping). With no matches, the cursor stays put.
func (m *diffViewModel) setSearch(query string) {
	m.searchQuery = query
	m.buildLines() // re-render with the new highlight
	m.computeSearchMatches()
	if len(m.searchMatches) == 0 {
		return
	}
	m.searchIdx = 0
	for i, idx := range m.searchMatches {
		if idx >= m.searchOrigin {
			m.searchIdx = i
			break
		}
	}
	m.cursor = m.searchMatches[m.searchIdx]
	m.ensureCursorVisible()
	m.ensureMatchVisible()
}

// clearSearch drops the active search and removes highlighting.
func (m *diffViewModel) clearSearch() {
	m.searchQuery = ""
	m.searchMatches = nil
	m.searchIdx = -1
	m.searchInput.Blur()
	m.buildLines()
}

// nextMatch moves the cursor to the next match, wrapping past the end.
func (m *diffViewModel) nextMatch() {
	if len(m.searchMatches) == 0 {
		return
	}
	m.searchIdx = (m.searchIdx + 1) % len(m.searchMatches)
	m.cursor = m.searchMatches[m.searchIdx]
	m.ensureCursorVisible()
	m.ensureMatchVisible()
}

// prevMatch moves the cursor to the previous match, wrapping past the start.
func (m *diffViewModel) prevMatch() {
	if len(m.searchMatches) == 0 {
		return
	}
	m.searchIdx = (m.searchIdx - 1 + len(m.searchMatches)) % len(m.searchMatches)
	m.cursor = m.searchMatches[m.searchIdx]
	m.ensureCursorVisible()
	m.ensureMatchVisible()
}

// ensureMatchVisible scrolls horizontally (when wrap is off) so the first
// occurrence of the query on the current match line is within the viewport. A
// small margin keeps a little context around the match.
func (m *diffViewModel) ensureMatchVisible() {
	if m.wrapEnabled || m.searchQuery == "" {
		return
	}
	if m.searchIdx < 0 || m.searchIdx >= len(m.searchMatches) {
		return
	}
	idx := m.searchMatches[m.searchIdx]
	if !m.isNavigable(idx) {
		return
	}
	cols := matchColumnRanges(m.lineRefs[idx].content, m.searchQuery)
	if len(cols) == 0 {
		return
	}
	avail := m.viewWidth()
	if avail < 1 {
		return
	}
	// Columns are within the content; the rendered line prepends a gutter.
	start := gutterWidth + cols[0][0]
	end := gutterWidth + cols[0][1]
	margin := hScrollStep
	switch {
	case start < m.hOffset:
		m.hOffset = start - margin
	case end > m.hOffset+avail:
		m.hOffset = end - avail + margin
	}
	if m.hOffset < 0 {
		m.hOffset = 0
	}
	if max := m.maxHScroll(); m.hOffset > max {
		m.hOffset = max
	}
}

// clickToAbsIdx converts a panel-relative click Y (0 = top border row + 1)
// to an absolute index into lines[], accounting for any visible input box and
// for wrapped display rows. Returns -1 if the click lands inside the input box.
func (m diffViewModel) clickToAbsIdx(clickY int) int {
	avail := m.viewWidth()
	if !m.commentInputActive {
		return m.lineAtRow(m.offset, clickY, avail)
	}
	if m.fileCommentInput {
		// File-level input box sits at the top; code lines render from line 0.
		if clickY < inputBoxHeight {
			return -1
		}
		return m.lineAtRow(0, clickY-inputBoxHeight, avail)
	}
	codeAbove := m.rowSpan(m.offset, m.cursor, avail)
	if clickY < codeAbove {
		return m.lineAtRow(m.offset, clickY, avail)
	}
	if clickY < codeAbove+inputBoxHeight {
		return -1 // inside the input box
	}
	// Below the input box — map back to lines[], skipping the box rows.
	nextIdx := m.cursor + 1
	if m.isCommentDisplayLine(nextIdx) {
		nextIdx++ // this display line was skipped in the render
	}
	return m.lineAtRow(nextIdx, clickY-codeAbove-inputBoxHeight, avail)
}

func renderDiffLine(l git.Line, marked bool, fillWidth int, filePath string, p themeColors, indentSize int, query, langOverride string) string {
	gutter := git.FormatGutter(l)
	if marked {
		content := l.Content
		// Pad content to fill the available width so background covers the line.
		gutterWidth := 6 // matches gutter style Width(6)
		contentWidth := fillWidth - gutterWidth
		if contentWidth > 0 && len(content) < contentWidth {
			content += strings.Repeat(" ", contentWidth-len(content))
		}
		switch l.Type {
		case git.LineAdded:
			return markGutterAdded.Render(gutter) + markAddedStyle.Render(content)
		case git.LineRemoved:
			return markGutterRemoved.Render(gutter) + markRemovedStyle.Render(content)
		case git.LineContext:
			return markGutterContext.Render(gutter) + markContextStyle.Render(content)
		}
		return l.Content
	}

	// Render the content with normal styling (syntax highlight when available,
	// otherwise the base diff-line style), then overlay search highlighting on
	// just the matched columns so the rest of the line keeps its colors.
	var gutterStyle lipgloss.Style
	var content string
	switch l.Type {
	case git.LineAdded:
		gutterStyle = addedGutterStyle
		if highlighted, ok := highlightLine(l.Content, filePath, p.addedBg, indentSize, langOverride); ok {
			content = highlighted
		} else {
			content = addedStyle.Render(addIndentGuides(l.Content, indentSize, p.addedBg))
		}
	case git.LineRemoved:
		gutterStyle = removedGutterStyle
		if highlighted, ok := highlightLine(l.Content, filePath, p.removedBg, indentSize, langOverride); ok {
			content = highlighted
		} else {
			content = removedStyle.Render(addIndentGuides(l.Content, indentSize, p.removedBg))
		}
	case git.LineContext:
		gutterStyle = contextGutterStyle
		if highlighted, ok := highlightLine(l.Content, filePath, nil, indentSize, langOverride); ok {
			content = highlighted
		} else {
			content = contextStyle.Render(addIndentGuides(l.Content, indentSize, nil))
		}
	default:
		return l.Content
	}

	if query != "" {
		if cols := matchColumnRanges(l.Content, query); len(cols) > 0 {
			content = overlaySearchHighlight(content, cols)
		}
	}
	return gutterStyle.Render(gutter) + content
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

// linePrefixRow returns the leading prefix for a single display row of a
// (possibly wrapped) logical line. The first row gets the full prefix
// (cursor/stripe); continuation rows keep the comment/mark stripe so the
// bookmark stays visible, but never the cursor caret.
func (m diffViewModel) linePrefixRow(absIdx int, focused bool, rowIdx int) string {
	if rowIdx == 0 {
		return m.linePrefix(absIdx, focused)
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
	if m.wrapEnabled {
		hOffset = 0 // horizontal scroll is meaningless with wrap on
	} else if max := m.maxHScroll(); hOffset > max {
		hOffset = max
	}

	// rowsFor returns the prefixed display rows for a logical line — one row
	// when wrap is off, possibly several when a wrapped line spans the width.
	rowsFor := func(absIdx int) []string {
		body := m.displayRows(absIdx, maxWidth, hOffset)
		out := make([]string, len(body))
		for i, s := range body {
			out[i] = m.linePrefixRow(absIdx, focused, i) + s
		}
		return out
	}

	// appendRows draws logical lines starting at `from` until the viewport has
	// `limit` rows (or the lines run out), returning the rows used. Each wrapped
	// continuation row counts toward the limit.
	var renderedLines []string
	appendRows := func(from, limit int) int {
		used := 0
		for absIdx := from; absIdx < len(m.lines) && used < limit; absIdx++ {
			for _, r := range rowsFor(absIdx) {
				if used >= limit {
					break
				}
				renderedLines = append(renderedLines, r)
				used++
			}
		}
		return used
	}

	inputBoxView := func() string {
		inputWidth := m.width - 4
		if inputWidth < 10 {
			inputWidth = 10
		}
		m.textInput.SetWidth(inputWidth - 4)
		return commentInputStyle.Width(inputWidth).Render(m.textInput.View())
	}

	if len(m.lines) == 0 {
		renderedLines = append(renderedLines, "No changes to display")
	} else if m.commentInputActive && m.fileCommentInput {
		// File-level comment: input box appears before any code lines.
		renderedLines = append(renderedLines, inputBoxView())
		appendRows(0, viewH-inputBoxHeight)
	} else if m.commentInputActive {
		// Inline comment: render lines [offset..cursor], then the input box,
		// then the lines after the cursor (skipping the existing comment
		// display row, which the box replaces while editing).
		used := 0
		for absIdx := m.offset; absIdx <= m.cursor && absIdx < len(m.lines); absIdx++ {
			for _, r := range rowsFor(absIdx) {
				renderedLines = append(renderedLines, r)
				used++
			}
		}
		nextIdx := m.cursor + 1
		if m.isCommentDisplayLine(nextIdx) {
			nextIdx++
		}
		renderedLines = append(renderedLines, inputBoxView())
		used += inputBoxHeight
		appendRows(nextIdx, viewH-used)
	} else {
		appendRows(m.offset, viewH)
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
	// m.height is the inner content height; lipgloss v2 includes borders in
	// Height(), so we pass m.height + 2 (top + bottom border).
	rendered := style.Width(m.width).Height(m.height + 2).MaxHeight(m.height + 2).Render(content)

	if m.fileReviewMode {
		// File review mode: centered filename in top border, clean bottom border.
		slider := ""
		if len(modeSlider) > 0 {
			slider = modeSlider[0]
		}
		if slider != "" {
			rendered = setBorderTitleCentered(rendered, fileStyle.Render(" "+slider+" "), focused)
		}
		if m.wrapEnabled {
			rendered = setBorderBottomRight(rendered, modeActiveStyle.Render(" Wrap "), focused)
		}
		return m.overlayScrollbar(rendered, focused)
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
	// Bottom-right footer: optional " Wrap " indicator followed by the
	// whitespace indicator. Both are right-aligned as one label.
	right := ""
	if m.wrapEnabled {
		right += modeActiveStyle.Render(" Wrap ")
	}
	if hideWhitespace {
		right += modeActiveStyle.Render(" Whitespace hidden ")
	} else {
		right += statusBarStyle.Render(" Whitespace ")
	}
	rendered = setBorderBottomRight(rendered, right, focused)
	return m.overlayScrollbar(rendered, focused)
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

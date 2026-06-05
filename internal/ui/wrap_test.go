package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/justincampbell/revise/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wrapModel builds a model where every navigable line is `width` columns wide,
// so wrapped row counts are predictable in tests. Gutter width is 6, so the
// content after the gutter is width-6 columns.
func wrapModel(lineCount, height, paneWidth, lineWidth int) diffViewModel {
	m := makeDiffViewModel(lineCount, height)
	m.width = paneWidth
	m.wrapEnabled = true
	for i := range m.lines {
		m.lines[i] = strings.Repeat("A", lineWidth)
	}
	return m
}

func TestDisplayRows_WrapOff_AlwaysSingleRow(t *testing.T) {
	m := makeDiffViewModel(1, 10)
	m.width = 20 // viewWidth = 17
	m.lines[0] = strings.Repeat("A", 40)
	rows := m.displayRows(0, m.viewWidth(), 0)
	assert.Len(t, rows, 1, "wrap off should never split a line")
}

func TestDisplayRows_WrapOn_SplitsAndIndents(t *testing.T) {
	m := makeDiffViewModel(1, 10)
	m.width = 26 // viewWidth = 23
	m.wrapEnabled = true
	m.lines[0] = strings.Repeat("A", 40)
	avail := m.viewWidth() // 23, gutter 6 → contentAvail 17, content 34 → ceil(34/17)=2
	rows := m.displayRows(0, avail, 0)

	require.Len(t, rows, 2)
	for _, r := range rows {
		assert.LessOrEqual(t, ansi.StringWidth(r), avail, "no wrapped row should exceed the available width")
	}
	// Continuation row is indented past the gutter so content aligns.
	assert.True(t, strings.HasPrefix(rows[1], strings.Repeat(" ", gutterWidth)),
		"continuation row should be indented by the gutter width")
}

func TestDisplayRows_WrapOn_ShortLineStaysSingle(t *testing.T) {
	m := makeDiffViewModel(1, 10)
	m.width = 40
	m.wrapEnabled = true
	m.lines[0] = "short"
	rows := m.displayRows(0, m.viewWidth(), 0)
	assert.Len(t, rows, 1)
}

func TestLineRows_CountsWrappedRows(t *testing.T) {
	// pane 26 → viewWidth 23, gutter 6 → contentAvail 17.
	m := wrapModel(1, 10, 26, 6+17*2) // content exactly 2 full rows
	assert.Equal(t, 2, m.lineRows(0, m.viewWidth()))
}

func TestToggleWrap_FlipsAndResetsHOffset(t *testing.T) {
	m := makeDiffViewModel(3, 10)
	m.hOffset = 12
	require.False(t, m.wrapEnabled)

	m.toggleWrap()
	assert.True(t, m.wrapEnabled)
	assert.Equal(t, 0, m.hOffset, "enabling wrap resets horizontal scroll")

	m.toggleWrap()
	assert.False(t, m.wrapEnabled)
}

func TestEnsureCursorVisible_WrapScrollsSoCursorFits(t *testing.T) {
	// viewH 4, each line 2 rows → only 2 logical lines fit at once.
	m := wrapModel(10, 4, 26, 6+17+1) // 6+18 → contentAvail 17 → ceil(18/17)=2 rows
	require.Equal(t, 2, m.lineRows(0, m.viewWidth()))

	m.cursor = 5
	m.ensureCursorVisible()
	assert.Equal(t, 4, m.offset, "offset should advance so the 2-row cursor line is visible")
}

func TestLineAtRow_WrapMapsRowsToLogicalLines(t *testing.T) {
	m := wrapModel(5, 10, 26, 6+17+1) // 2 rows per line
	avail := m.viewWidth()
	assert.Equal(t, 0, m.lineAtRow(0, 0, avail))
	assert.Equal(t, 0, m.lineAtRow(0, 1, avail))
	assert.Equal(t, 1, m.lineAtRow(0, 2, avail))
	assert.Equal(t, 2, m.lineAtRow(0, 4, avail))
}

func TestClickToAbsIdx_WrapNoInputBox(t *testing.T) {
	m := wrapModel(5, 10, 26, 6+17+1) // 2 rows per line
	m.offset = 0
	// Display row 2 is the first row of logical line 1.
	assert.Equal(t, 1, m.clickToAbsIdx(2))
}

func TestBottomOffset_Wrap(t *testing.T) {
	m := wrapModel(10, 4, 26, 6+17+1) // 2 rows per line, viewH 4
	assert.Equal(t, 8, m.bottomOffset())
}

func TestLastVisibleLine_Wrap(t *testing.T) {
	m := wrapModel(10, 4, 26, 6+17+1) // 2 rows per line, viewH 4
	m.offset = 2
	assert.Equal(t, 3, m.lastVisibleLine())
}

func TestRender_WrapKeepsRowsWithinWidthAndShowsIndicator(t *testing.T) {
	m := newDiffViewModel()
	m.width = 30
	m.height = 10
	m.file = &git.FileDiff{
		Path: "doc.md",
		Hunks: []git.Hunk{{
			Header: "@@ -1,1 +1,1 @@",
			Lines:  []git.Line{{Type: git.LineContext, Content: strings.Repeat("word ", 30), NewNum: 1}},
		}},
	}
	m.buildLines()
	m.goToFirstNavigable()
	m.wrapEnabled = true

	out := m.render(true, 3, false)
	assert.Contains(t, ansi.Strip(out), "Wrap", "footer should show the wrap indicator")
	for _, line := range strings.Split(out, "\n") {
		assert.LessOrEqual(t, ansi.StringWidth(line), m.width,
			"every rendered row (incl. borders) must fit the pane width")
	}
}

func TestRender_WrapFileReviewShowsIndicator(t *testing.T) {
	m := newDiffViewModel()
	m.fileReviewMode = true
	m.width = 30
	m.height = 8
	m.file = &git.FileDiff{
		Path: "doc.md",
		Hunks: []git.Hunk{{
			Lines: []git.Line{{Type: git.LineContext, Content: strings.Repeat("word ", 30), NewNum: 1}},
		}},
	}
	m.buildLines()
	m.goToFirstNavigable()
	m.wrapEnabled = true

	out := ansi.Strip(m.render(true, 3, false, "doc.md"))
	assert.Contains(t, out, "Wrap")
}

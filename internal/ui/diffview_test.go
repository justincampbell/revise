package ui

import (
	"testing"

	"github.com/justincampbell/revise/internal/git"
	"github.com/stretchr/testify/assert"
)

func makeDiffViewModel(lineCount, height int) diffViewModel {
	m := newDiffViewModel()
	m.height = height
	m.lines = make([]string, lineCount)
	m.lineRefs = make([]*lineRef, lineCount)
	for i := range m.lines {
		m.lines[i] = "line"
	}
	return m
}

func TestDiffViewScrollDown_Clamps(t *testing.T) {
	m := makeDiffViewModel(10, 6) // viewHeight = 4, max offset = 6
	m.scrollDown(100)
	assert.Equal(t, 6, m.offset)
}

func TestDiffViewScrollUp_Clamps(t *testing.T) {
	m := makeDiffViewModel(10, 6)
	m.offset = 3
	m.scrollUp(100)
	assert.Equal(t, 0, m.offset)
}

func TestDiffViewGoToTop(t *testing.T) {
	m := makeDiffViewModel(10, 6)
	m.offset = 5
	m.goToTop()
	assert.Equal(t, 0, m.offset)
}

func TestDiffViewGoToBottom(t *testing.T) {
	m := makeDiffViewModel(10, 6) // viewHeight = 4, max = 6
	m.goToBottom()
	assert.Equal(t, 6, m.offset)
}

func TestDiffViewGoToBottom_ShortContent(t *testing.T) {
	m := makeDiffViewModel(2, 10) // content fits, max = 0
	m.goToBottom()
	assert.Equal(t, 0, m.offset)
}

func TestDiffViewViewHeight(t *testing.T) {
	m := newDiffViewModel()
	m.height = 20
	assert.Equal(t, 18, m.viewHeight())
}

func TestDiffViewViewHeight_MinimumOne(t *testing.T) {
	m := newDiffViewModel()
	m.height = 0
	assert.Equal(t, 1, m.viewHeight())
}

func TestFormatGutter_Added(t *testing.T) {
	l := git.Line{Type: git.LineAdded, NewNum: 42}
	assert.Equal(t, "       42 ", formatGutter(l))
}

func TestFormatGutter_Removed(t *testing.T) {
	l := git.Line{Type: git.LineRemoved, OldNum: 7}
	assert.Equal(t, "   7      ", formatGutter(l))
}

func TestFormatGutter_Context(t *testing.T) {
	l := git.Line{Type: git.LineContext, OldNum: 3, NewNum: 5}
	assert.Equal(t, "   3    5 ", formatGutter(l))
}

func TestDiffViewCursorMovesDown(t *testing.T) {
	m := makeDiffViewModel(10, 6) // viewHeight = 4
	m.moveCursorDown(1)
	assert.Equal(t, 1, m.cursor)
	assert.Equal(t, 0, m.offset)
}

func TestDiffViewCursorScrollsWhenNeeded(t *testing.T) {
	m := makeDiffViewModel(10, 6) // viewHeight = 4
	// cursor at 5 means offset must be at least 5-3=2
	m.moveCursorDown(5)
	assert.Equal(t, 5, m.cursor)
	assert.Equal(t, 2, m.offset)
}

func TestDiffViewCursorClampsAtBottom(t *testing.T) {
	m := makeDiffViewModel(10, 6)
	m.moveCursorDown(100)
	assert.Equal(t, 9, m.cursor)
}

func TestDiffViewCursorMovesUp(t *testing.T) {
	m := makeDiffViewModel(10, 6)
	m.cursor = 5
	m.offset = 2
	m.moveCursorUp(1)
	assert.Equal(t, 4, m.cursor)
	assert.Equal(t, 2, m.offset)
}

func TestDiffViewCursorScrollsUpWhenNeeded(t *testing.T) {
	m := makeDiffViewModel(10, 6)
	m.cursor = 5
	m.offset = 5
	m.moveCursorUp(3)
	assert.Equal(t, 2, m.cursor)
	assert.Equal(t, 2, m.offset)
}

func TestDiffViewCursorClampsAtTop(t *testing.T) {
	m := makeDiffViewModel(10, 6)
	m.cursor = 3
	m.moveCursorUp(100)
	assert.Equal(t, 0, m.cursor)
	assert.Equal(t, 0, m.offset)
}

func TestCursorRef_OnHeaderLine(t *testing.T) {
	m := newDiffViewModel()
	m.file = &git.FileDiff{Path: "foo.go"}
	m.buildLines()
	m.cursor = 0 // file header line
	assert.Nil(t, m.cursorRef())
}

func TestCursorRef_OnCodeLine(t *testing.T) {
	m := newDiffViewModel()
	m.file = &git.FileDiff{
		Path: "foo.go",
		Hunks: []git.Hunk{{
			Header: "@@ -1,1 +1,1 @@",
			Lines: []git.Line{{
				Type: git.LineAdded, Content: "hello", NewNum: 1,
			}},
		}},
	}
	m.buildLines()
	// lines: [0: header, 1: blank, 2: hunk, 3: code, 4: blank]
	m.cursor = 3
	ref := m.cursorRef()
	assert.NotNil(t, ref)
	assert.Equal(t, 1, ref.commentLineNum())
}

func TestLineRef_CommentLineNum_Added(t *testing.T) {
	r := lineRef{newNum: 10, oldNum: 0, lineType: git.LineAdded}
	assert.Equal(t, 10, r.commentLineNum())
}

func TestLineRef_CommentLineNum_Removed(t *testing.T) {
	r := lineRef{newNum: 0, oldNum: 5, lineType: git.LineRemoved}
	assert.Equal(t, 5, r.commentLineNum())
}

func TestLineRef_CommentLineNum_Context(t *testing.T) {
	r := lineRef{newNum: 10, oldNum: 9, lineType: git.LineContext}
	assert.Equal(t, 10, r.commentLineNum())
}

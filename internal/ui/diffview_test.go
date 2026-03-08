package ui

import (
	"testing"

	"github.com/justincampbell/revise/internal/git"
	"github.com/stretchr/testify/assert"
)

// makeDiffViewModel creates a model with lineCount navigable (code) lines.
func makeDiffViewModel(lineCount, height int) diffViewModel {
	m := newDiffViewModel()
	m.height = height
	m.lines = make([]string, lineCount)
	m.lineRefs = make([]*lineRef, lineCount)
	for i := range m.lines {
		m.lines[i] = "line"
		m.lineRefs[i] = &lineRef{newNum: i + 1, lineType: git.LineContext}
	}
	return m
}

func TestDiffViewScrollDown_Clamps(t *testing.T) {
	m := makeDiffViewModel(10, 6) // viewHeight = 5, max offset = 5
	m.scrollDown(100)
	assert.Equal(t, 5, m.offset)
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
	m := makeDiffViewModel(10, 6) // viewHeight = 5, max = 5
	m.goToBottom()
	assert.Equal(t, 5, m.offset)
}

func TestDiffViewGoToBottom_ShortContent(t *testing.T) {
	m := makeDiffViewModel(2, 10) // content fits, max = 0
	m.goToBottom()
	assert.Equal(t, 0, m.offset)
}

func TestDiffViewViewHeight(t *testing.T) {
	m := newDiffViewModel()
	m.height = 20
	assert.Equal(t, 19, m.viewHeight())
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
	m := makeDiffViewModel(10, 6) // viewHeight = 5
	m.moveCursorDown(5)
	assert.Equal(t, 5, m.cursor)
	assert.Equal(t, 1, m.offset)
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

func TestDiffViewCursorSkipsNilRefLines(t *testing.T) {
	// Build a model directly with known nil/non-nil refs to test skipping.
	m := newDiffViewModel()
	m.height = 20
	m.lines = []string{"hdr", "blank", "hunk", "code-a", "code-b", "blank"}
	m.lineRefs = []*lineRef{
		nil,
		nil,
		nil,
		{newNum: 1, lineType: git.LineContext},
		{newNum: 2, lineType: git.LineContext},
		nil,
	}
	m.goToFirstNavigable()
	assert.Equal(t, 3, m.cursor)

	// One step down → code-b (index 4), skipping nothing.
	m.moveCursorDown(1)
	assert.Equal(t, 4, m.cursor)

	// Another step down → can't go further (index 5 is nil, nothing navigable after 4).
	m.moveCursorDown(1)
	assert.Equal(t, 4, m.cursor)

	// One step up → back to code-a (index 3).
	m.moveCursorUp(1)
	assert.Equal(t, 3, m.cursor)

	// Step up from first navigable → stays at 3 (indices 0-2 are nil).
	m.moveCursorUp(1)
	assert.Equal(t, 3, m.cursor)
}

func TestCursorRef_OnHeaderLine(t *testing.T) {
	m := newDiffViewModel()
	m.file = &git.FileDiff{
		Path: "foo.go",
		Hunks: []git.Hunk{{
			Header: "@@ -1,1 +1,1 @@", NewStart: 1,
			Lines: []git.Line{{Type: git.LineAdded, Content: "hello", NewNum: 1}},
		}},
	}
	m.buildLines()
	m.cursor = 0 // hunk header line (non-navigable)
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
	// lines: [0: hunk, 1: code, 2: blank]
	m.cursor = 1
	ref := m.cursorRef()
	assert.NotNil(t, ref)
	key := ref.commentKey("foo.go")
	assert.Equal(t, 1, key.lineNum)
	assert.False(t, key.isOld)
}

func TestFormatHunkHeader_WithContext(t *testing.T) {
	h := git.Hunk{Header: "@@ -1,5 +7,7 @@ func foo() {", NewStart: 7}
	assert.Equal(t, "func foo() {", formatHunkHeader(h))
}

func TestFormatHunkHeader_NoContext(t *testing.T) {
	h := git.Hunk{Header: "@@ -1,5 +7,7 @@", NewStart: 7}
	assert.Equal(t, "@@ line 7", formatHunkHeader(h))
}

func TestFormatHunkHeader_ContextWithTrailingSpace(t *testing.T) {
	h := git.Hunk{Header: "@@ -1,5 +7,7 @@   func bar()  ", NewStart: 7}
	assert.Equal(t, "func bar()", formatHunkHeader(h))
}

func TestLineRef_CommentKey_Added(t *testing.T) {
	r := lineRef{newNum: 10, oldNum: 0, lineType: git.LineAdded}
	key := r.commentKey("foo.go")
	assert.Equal(t, 10, key.lineNum)
	assert.False(t, key.isOld)
}

func TestLineRef_CommentKey_Removed(t *testing.T) {
	r := lineRef{newNum: 0, oldNum: 5, lineType: git.LineRemoved}
	key := r.commentKey("foo.go")
	assert.Equal(t, 5, key.lineNum)
	assert.True(t, key.isOld)
}

func TestLineRef_CommentKey_Context(t *testing.T) {
	r := lineRef{newNum: 10, oldNum: 9, lineType: git.LineContext}
	key := r.commentKey("foo.go")
	assert.Equal(t, 10, key.lineNum)
	assert.False(t, key.isOld)
}

func TestLineRef_CommentKey_NoCollision_RemovedAndAdded(t *testing.T) {
	// Removed line 5 and added/context line 5 must produce different keys.
	removed := lineRef{oldNum: 5, lineType: git.LineRemoved}
	added := lineRef{newNum: 5, lineType: git.LineAdded}
	assert.NotEqual(t, removed.commentKey("f"), added.commentKey("f"))
}

func TestClickToAbsIdx_NoInputBox(t *testing.T) {
	m := makeDiffViewModel(10, 6)
	m.offset = 2
	assert.Equal(t, 5, m.clickToAbsIdx(3))
}

func TestClickToAbsIdx_WithInputBox_Above(t *testing.T) {
	m := makeDiffViewModel(10, 10)
	m.offset = 0
	m.cursor = 3
	m.commentInputActive = true
	// Click on the cursor line itself (clickY=3, codeAbove=4, so 3 < 4 → above/at cursor)
	assert.Equal(t, 3, m.clickToAbsIdx(3))
}

func TestClickToAbsIdx_WithInputBox_Inside(t *testing.T) {
	m := makeDiffViewModel(10, 10)
	m.offset = 0
	m.cursor = 3
	m.commentInputActive = true
	// Click in the input box (clickY=4, codeAbove=4, inputBoxHeight=3 → 4 < 4+3=7)
	assert.Equal(t, -1, m.clickToAbsIdx(4))
}

func TestClickToAbsIdx_WithInputBox_Below(t *testing.T) {
	m := makeDiffViewModel(10, 10)
	m.offset = 0
	m.cursor = 3
	m.commentInputActive = true
	// Click below box (clickY=7, codeAbove=4, inputBoxHeight=3 → nextIdx=4, belowClick=0)
	assert.Equal(t, 4, m.clickToAbsIdx(7))
}

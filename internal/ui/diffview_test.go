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
	m := makeDiffViewModel(10, 6) // viewHeight = 6, max offset = 4
	m.scrollDown(100)
	assert.Equal(t, 4, m.offset)
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
	m := makeDiffViewModel(10, 6) // viewHeight = 6, max = 4
	m.goToBottom()
	assert.Equal(t, 4, m.offset)
}

func TestDiffViewGoToBottom_ShortContent(t *testing.T) {
	m := makeDiffViewModel(2, 10) // content fits, max = 0
	m.goToBottom()
	assert.Equal(t, 0, m.offset)
}

func TestDiffViewViewHeight(t *testing.T) {
	m := newDiffViewModel()
	m.height = 20
	assert.Equal(t, 20, m.viewHeight())
}

func TestDiffViewViewHeight_MinimumOne(t *testing.T) {
	m := newDiffViewModel()
	m.height = 0
	assert.Equal(t, 1, m.viewHeight())
}

func TestFormatGutter_Added(t *testing.T) {
	l := git.Line{Type: git.LineAdded, NewNum: 42}
	assert.Equal(t, "   42 ", formatGutter(l))
}

func TestFormatGutter_Removed(t *testing.T) {
	l := git.Line{Type: git.LineRemoved, OldNum: 7}
	assert.Equal(t, "    7 ", formatGutter(l))
}

func TestFormatGutter_Context(t *testing.T) {
	l := git.Line{Type: git.LineContext, OldNum: 3, NewNum: 5}
	assert.Equal(t, "    5 ", formatGutter(l))
}

func TestDiffViewCursorMovesDown(t *testing.T) {
	m := makeDiffViewModel(10, 6) // viewHeight = 6
	m.moveCursorDown(1)
	assert.Equal(t, 1, m.cursor)
	assert.Equal(t, 0, m.offset)
}

func TestDiffViewCursorScrollsWhenNeeded(t *testing.T) {
	m := makeDiffViewModel(10, 6) // viewHeight = 6
	m.moveCursorDown(5)
	assert.Equal(t, 5, m.cursor)
	assert.Equal(t, 0, m.offset)
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

func TestCursorRef_OnHunkHeaderLine(t *testing.T) {
	m := newDiffViewModel()
	m.file = &git.FileDiff{
		Path: "foo.go",
		Hunks: []git.Hunk{{
			Header: "@@ -1,1 +1,1 @@ func foo()",
			Lines:  []git.Line{{Type: git.LineAdded, Content: "hello", NewNum: 1}},
		}},
	}
	m.buildLines()
	m.cursor = 0 // hunk header line (nil ref)
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
	// lines: [0: code, 1: blank] (no hunk context header)
	m.cursor = 0
	ref := m.cursorRef()
	assert.NotNil(t, ref)
	key := ref.commentKey("foo.go")
	assert.Equal(t, 1, key.lineNum)
	assert.False(t, key.isOld)
}

func TestLinePrefix_NoFileSelected(t *testing.T) {
	m := newDiffViewModel()
	// m.file is nil — no file selected
	assert.Equal(t, " ", m.linePrefix(0))
}

func TestLinePrefix_WithFileSelected(t *testing.T) {
	m := newDiffViewModel()
	m.file = &git.FileDiff{Path: "foo.go"}
	m.cursor = 2
	assert.Equal(t, " ", m.linePrefix(0))
	assert.Contains(t, m.linePrefix(2), "▶")
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

func TestBuildLines_NoFileHeader(t *testing.T) {
	m := newDiffViewModel()
	m.file = &git.FileDiff{
		Path: "foo.go",
		Hunks: []git.Hunk{{
			Header: "@@ -1,1 +1,1 @@",
			Lines:  []git.Line{{Type: git.LineAdded, Content: "hello", NewNum: 1}},
		}},
	}
	m.buildLines()
	assert.Contains(t, m.lines[0], "hello")
	assert.NotContains(t, m.lines[0], "foo.go")
}

func TestHunkContext_ExtractsTrailingContext(t *testing.T) {
	got := hunkContext("@@ -10,6 +12,7 @@ func renderStatusBar() string {")
	assert.Equal(t, "func renderStatusBar() string {", got)
}

func TestHunkContext_NoAtAt(t *testing.T) {
	got := hunkContext("some random text")
	assert.Equal(t, "some random text", got)
}

func TestHunkContext_EmptyContext(t *testing.T) {
	got := hunkContext("@@ -1,1 +1,1 @@")
	assert.Equal(t, "", got)
}

func TestDiffView_TitleInBorder(t *testing.T) {
	m := newDiffViewModel()
	m.width = 40
	m.height = 10
	m.file = &git.FileDiff{
		Path: "foo.go",
		Hunks: []git.Hunk{{
			Header: "@@ -1,1 +1,1 @@",
			Lines:  []git.Line{{Type: git.LineAdded, Content: "hello", NewNum: 1}},
		}},
	}
	m.buildLines()
	rendered := m.render(true)
	assert.Contains(t, rendered, "foo.go")
}

func TestDiffView_RenamedTitleInBorder(t *testing.T) {
	m := newDiffViewModel()
	m.width = 60
	m.height = 10
	m.file = &git.FileDiff{
		Path:    "new.go",
		OldPath: "old.go",
		Hunks: []git.Hunk{{
			Header: "@@ -1,1 +1,1 @@",
			Lines:  []git.Line{{Type: git.LineAdded, Content: "hello", NewNum: 1}},
		}},
	}
	m.buildLines()
	rendered := m.render(true)
	assert.Contains(t, rendered, "old.go → new.go")
}

func TestDiffView_RenderShowsCenteredFooterTotals(t *testing.T) {
	m := newDiffViewModel()
	m.width = 50
	m.height = 8
	m.file = &git.FileDiff{
		Path: "foo.go",
		Hunks: []git.Hunk{{
			Header: "@@ -1,1 +1,2 @@",
			Lines: []git.Line{
				{Type: git.LineAdded, Content: "a", NewNum: 1},
				{Type: git.LineRemoved, Content: "b", OldNum: 1},
				{Type: git.LineAdded, Content: "c", NewNum: 2},
			},
		}},
	}
	m.buildLines()
	rendered := m.render(true)
	assert.Contains(t, rendered, "+2/-1")
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

// makeDiffViewModelWithHunks creates a diffViewModel with multiple hunks for testing navigation.
func makeDiffViewModelWithHunks() diffViewModel {
	m := newDiffViewModel()
	m.height = 40
	m.file = &git.FileDiff{
		Path: "test.go",
		Hunks: []git.Hunk{
			{
				Header: "@@ -1,3 +1,4 @@ func A()",
				Lines: []git.Line{
					{Type: git.LineContext, Content: "ctx1", OldNum: 1, NewNum: 1},
					{Type: git.LineAdded, Content: "added1", NewNum: 2},
					{Type: git.LineContext, Content: "ctx2", OldNum: 2, NewNum: 3},
				},
			},
			{
				Header: "@@ -10,3 +11,4 @@ func B()",
				Lines: []git.Line{
					{Type: git.LineContext, Content: "ctx3", OldNum: 10, NewNum: 11},
					{Type: git.LineRemoved, Content: "removed1", OldNum: 11},
					{Type: git.LineAdded, Content: "added2", NewNum: 12},
					{Type: git.LineContext, Content: "ctx4", OldNum: 12, NewNum: 13},
				},
			},
			{
				Header: "@@ -20,2 +22,3 @@ func C()",
				Lines: []git.Line{
					{Type: git.LineContext, Content: "ctx5", OldNum: 20, NewNum: 22},
					{Type: git.LineAdded, Content: "added3", NewNum: 23},
				},
			},
		},
	}
	m.buildLines()
	m.goToFirstNavigable()
	return m
}

func TestNextHunk_MovesToFirstLineOfNextHunk(t *testing.T) {
	m := makeDiffViewModelWithHunks()
	// Cursor starts on first navigable line of hunk 0
	assert.Equal(t, 1, m.lineRefs[m.cursor].newNum) // ctx1, newNum=1
	m.nextHunk()
	// Should be on first code line of hunk 1
	assert.Equal(t, 11, m.lineRefs[m.cursor].newNum) // ctx3, newNum=11
}

func TestNextHunk_FromMiddleOfHunk(t *testing.T) {
	m := makeDiffViewModelWithHunks()
	m.moveCursorDown(1) // move to added1
	assert.Equal(t, 2, m.lineRefs[m.cursor].newNum)
	m.nextHunk()
	assert.Equal(t, 11, m.lineRefs[m.cursor].newNum) // first line of hunk 1
}

func TestNextHunk_FromLastHunk_StaysAtBottom(t *testing.T) {
	m := makeDiffViewModelWithHunks()
	// Go to last hunk
	m.nextHunk()
	m.nextHunk()
	assert.Equal(t, 22, m.lineRefs[m.cursor].newNum) // first line of hunk 2
	m.nextHunk()
	// Should stay on last hunk's first line (no more hunks)
	assert.Equal(t, 22, m.lineRefs[m.cursor].newNum)
}

func TestPrevHunk_MovesToFirstLineOfPrevHunk(t *testing.T) {
	m := makeDiffViewModelWithHunks()
	m.nextHunk()
	m.nextHunk()
	assert.Equal(t, 22, m.lineRefs[m.cursor].newNum)
	m.prevHunk()
	assert.Equal(t, 11, m.lineRefs[m.cursor].newNum)
}

func TestPrevHunk_FromFirstHunk_StaysAtTop(t *testing.T) {
	m := makeDiffViewModelWithHunks()
	m.prevHunk()
	assert.Equal(t, 1, m.lineRefs[m.cursor].newNum) // still on first line
}

func TestPrevHunk_FromMiddleOfHunk_GoesToStartOfCurrentHunk(t *testing.T) {
	m := makeDiffViewModelWithHunks()
	m.nextHunk()            // go to hunk 1
	m.moveCursorDown(2)     // move into middle of hunk 1
	assert.Equal(t, 12, m.lineRefs[m.cursor].newNum)
	m.prevHunk()
	// Should go to start of hunk 1 (since we're in the middle of it)
	assert.Equal(t, 11, m.lineRefs[m.cursor].newNum)
}

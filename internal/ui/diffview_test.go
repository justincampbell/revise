package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/justincampbell/revise/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	// lines: [0: source tag, 1: code, 2: blank]
	m.goToFirstNavigable()
	ref := m.cursorRef()
	assert.NotNil(t, ref)
	key := ref.commentKey("foo.go")
	assert.Equal(t, 1, key.lineNum)
	assert.False(t, key.isOld)
}

func TestLinePrefix_NoFileSelected(t *testing.T) {
	m := newDiffViewModel()
	// m.file is nil — no file selected
	assert.Equal(t, " ", m.linePrefix(0, true))
}

func TestLinePrefix_WithFileSelected_Focused(t *testing.T) {
	m := newDiffViewModel()
	m.file = &git.FileDiff{Path: "foo.go"}
	m.cursor = 2
	assert.Equal(t, " ", m.linePrefix(0, true))
	assert.Contains(t, m.linePrefix(2, true), "▶")
}

func TestLinePrefix_WithFileSelected_NotFocused(t *testing.T) {
	m := newDiffViewModel()
	m.file = &git.FileDiff{Path: "foo.go"}
	m.cursor = 2
	assert.Equal(t, " ", m.linePrefix(0, false))
	assert.Equal(t, " ", m.linePrefix(2, false)) // cursor hidden when not focused
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
	// No line should contain the file path (title is in the border, not inline)
	for _, line := range m.lines {
		assert.NotContains(t, line, "foo.go")
	}
}

func TestRenderHunkHeader_ShowsSourceTag(t *testing.T) {
	tests := []struct {
		source git.HunkSource
		tag    string
	}{
		{git.SourceBranch, "[branch]"},
		{git.SourceStaged, "[staged]"},
		{git.SourceUnstaged, "[unstaged]"},
	}
	for _, tt := range tests {
		t.Run(string(tt.source), func(t *testing.T) {
			h := git.Hunk{
				Header: "@@ -1,3 +1,4 @@ func foo()",
				Source: tt.source,
			}
			got := renderHunkHeader(h)
			assert.Contains(t, got, tt.tag)
			assert.Contains(t, got, "func foo()")
		})
	}
}

func TestRenderHunkHeader_EmptyContext_StillShowsTag(t *testing.T) {
	h := git.Hunk{
		Header: "@@ -1,1 +1,1 @@",
		Source: git.SourceStaged,
	}
	got := renderHunkHeader(h)
	assert.Contains(t, got, "[staged]")
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
	rendered := m.render(true, 3, false)
	assert.Contains(t, rendered, "foo.go")
}

func TestDiffView_ModeSliderInBorder(t *testing.T) {
	m := newDiffViewModel()
	m.width = 60
	m.height = 10
	m.file = &git.FileDiff{
		Path: "foo.go",
		Hunks: []git.Hunk{{
			Header: "@@ -1,1 +1,1 @@",
			Lines:  []git.Line{{Type: git.LineAdded, Content: "hello", NewNum: 1}},
		}},
	}
	m.buildLines()
	rendered := m.render(true, 3, false, "Branch+Staged+Unstaged")
	assert.Contains(t, rendered, "Branch+Staged+Unstaged")
	assert.Contains(t, rendered, "foo.go", "file title should still appear")
}

func TestDiffView_EmptyModeSliderOmitted(t *testing.T) {
	m := newDiffViewModel()
	m.width = 60
	m.height = 10
	m.file = &git.FileDiff{
		Path: "foo.go",
		Hunks: []git.Hunk{{
			Header: "@@ -1,1 +1,1 @@",
			Lines:  []git.Line{{Type: git.LineAdded, Content: "hello", NewNum: 1}},
		}},
	}
	m.buildLines()
	// Empty mode slider should not change the border
	rendered := m.render(true, 3, false, "")
	assert.Contains(t, rendered, "foo.go")
	assert.NotContains(t, rendered, "Branch")
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
	rendered := m.render(true, 3, false)
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
	rendered := m.render(true, 3, false)
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

func TestNextHunk_FromLastHunk_JumpsToEnd(t *testing.T) {
	m := makeDiffViewModelWithHunks()
	// Go to last hunk
	m.nextHunk()
	m.nextHunk()
	assert.Equal(t, 22, m.lineRefs[m.cursor].newNum) // first line of hunk 2
	m.nextHunk()
	// Should jump to the last navigable line (past the last hunk)
	assert.Equal(t, 23, m.lineRefs[m.cursor].newNum)
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

func TestNextHunk_FromLastHunk_JumpsPastToLastLine(t *testing.T) {
	m := makeDiffViewModelWithHunks()
	// Go to last hunk
	m.nextHunk()
	m.nextHunk()
	assert.Equal(t, 22, m.lineRefs[m.cursor].newNum) // first line of hunk 2
	m.nextHunk()
	// Should jump to the last navigable line (past the last hunk)
	assert.Equal(t, 23, m.lineRefs[m.cursor].newNum) // added3, last navigable line
}

func TestPrevHunk_FromFirstLineOfFirstHunk_JumpsToTop(t *testing.T) {
	m := makeDiffViewModelWithHunks()
	// Cursor is already on first navigable line (first line of first hunk)
	assert.Equal(t, 1, m.lineRefs[m.cursor].newNum)
	// prevHunk should be a no-op since we're already at the start of the first hunk
	m.prevHunk()
	assert.Equal(t, 1, m.lineRefs[m.cursor].newNum)
}

func TestCurrentHunkIndex_FirstHunk(t *testing.T) {
	m := makeDiffViewModelWithHunks()
	// Cursor starts on first navigable line of hunk 0
	assert.Equal(t, 0, m.currentHunkIndex())
}

func TestCurrentHunkIndex_SecondHunk(t *testing.T) {
	m := makeDiffViewModelWithHunks()
	m.nextHunk()
	assert.Equal(t, 1, m.currentHunkIndex())
}

func TestCurrentHunkIndex_ThirdHunk(t *testing.T) {
	m := makeDiffViewModelWithHunks()
	m.nextHunk()
	m.nextHunk()
	assert.Equal(t, 2, m.currentHunkIndex())
}

func TestCurrentHunkIndex_MiddleOfHunk(t *testing.T) {
	m := makeDiffViewModelWithHunks()
	m.nextHunk()
	m.moveCursorDown(2) // middle of hunk 1
	assert.Equal(t, 1, m.currentHunkIndex())
}

func TestCurrentHunkIndex_NoFile(t *testing.T) {
	m := newDiffViewModel()
	m.height = 10
	assert.Equal(t, -1, m.currentHunkIndex())
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

// makePlainBottomBorder builds a minimal two-line "rendered" string that
// setBorderBottomCounts can operate on: one content line + one bottom border.
func makePlainBottomBorder(width int) string {
	top := strings.Repeat("x", width)
	bottom := "╰" + strings.Repeat("─", width-2) + "╯"
	return top + "\n" + bottom
}

// stripANSI removes all ANSI escape sequences from s so we can inspect
// plain character positions.
func stripANSI(s string) string {
	return ansi.Strip(s)
}

func TestSetBorderBottomCounts_SlashColumnFixed(t *testing.T) {
	// The slash should always appear at the same column regardless of
	// digit counts in the added/removed numbers.
	const paneWidth = 60

	cases := []struct {
		name    string
		added   int
		removed int
	}{
		{"single digits", 3, 7},
		{"double/single", 21, 7},
		{"quad/triple", 1234, 768},
		{"large/large", 9999, 9999},
		{"zero/zero", 0, 0},
		{"single/quad", 1, 1234},
	}

	rendered := makePlainBottomBorder(paneWidth)
	contentWidth := paneWidth - 2 // minus corners
	expectedSlashCol := 1 + contentWidth/2 // 1 for ╰ corner

	var slashCols []int

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := setBorderBottomCounts(rendered, tc.added, tc.removed, false)
			// Extract bottom line
			lastNl := strings.LastIndexByte(result, '\n')
			require.NotEqual(t, -1, lastNl)
			bottomLine := result[lastNl+1:]

			plain := stripANSI(bottomLine)
			plainRunes := []rune(plain)

			// Find the slash position
			slashIdx := -1
			for i, r := range plainRunes {
				if r == '/' {
					slashIdx = i
					break
				}
			}
			require.NotEqual(t, -1, slashIdx, "slash not found in bottom border for case %s: %q", tc.name, plain)

			// Slash should be at the center column
			assert.Equal(t, expectedSlashCol, slashIdx,
				"slash column mismatch for %s: got col %d, want %d\nborder: %q", tc.name, slashIdx, expectedSlashCol, plain)

			slashCols = append(slashCols, slashIdx)
		})
	}

	// Additionally verify all cases placed the slash at the same column
	for i := 1; i < len(slashCols); i++ {
		assert.Equal(t, slashCols[0], slashCols[i],
			"slash column differs between case 0 and case %d", i)
	}
}

func TestSetBorderBottomCounts_LeadingAndTrailingSpaces(t *testing.T) {
	// The count block should have exactly one leading space before +N
	// and one trailing space after -N.
	const paneWidth = 60

	cases := []struct {
		name    string
		added   int
		removed int
	}{
		{"small", 2, 1},
		{"medium", 21, 7},
		{"large", 1234, 768},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rendered := makePlainBottomBorder(paneWidth)
			result := setBorderBottomCounts(rendered, tc.added, tc.removed, false)

			lastNl := strings.LastIndexByte(result, '\n')
			require.NotEqual(t, -1, lastNl)
			bottomLine := result[lastNl+1:]
			plain := stripANSI(bottomLine)

			// Find the count block by locating the + and the last digit before ╯/─
			addedStr := "+" + strings.TrimLeft(strings.Split(plain[strings.Index(plain, "+"):], "/")[0], "+")
			_ = addedStr

			// Simpler: find " +N" pattern — there must be a space before the +
			plusIdx := strings.Index(plain, "+")
			require.NotEqual(t, -1, plusIdx, "'+' not found: %q", plain)
			require.Greater(t, plusIdx, 0, "'+' at start of line")
			assert.Equal(t, ' ', rune(plain[plusIdx-1]),
				"expected space before '+' in %q", plain)

			// Find trailing space after -N: locate the last digit of -N block
			slashIdx := strings.Index(plain, "/")
			require.NotEqual(t, -1, slashIdx)
			// After slash, find the - sign and then the digits
			afterSlash := plain[slashIdx+1:]
			require.True(t, len(afterSlash) > 0 && afterSlash[0] == '-',
				"expected '-' after slash: %q", plain)
			// Find end of digits after -
			endOfDigits := 1 // skip '-'
			for endOfDigits < len(afterSlash) && afterSlash[endOfDigits] >= '0' && afterSlash[endOfDigits] <= '9' {
				endOfDigits++
			}
			require.Less(t, endOfDigits, len(afterSlash), "no space after removed count: %q", plain)
			assert.Equal(t, ' ', rune(afterSlash[endOfDigits]),
				"expected space after removed digits in %q", plain)
		})
	}
}

func TestSetBorderBottomCounts_ColorAndNoColorProduceSameLayout(t *testing.T) {
	// The visual (stripped) layout must be identical whether or not
	// styles produce ANSI codes.
	const paneWidth = 50

	rendered := makePlainBottomBorder(paneWidth)

	// Both focused and unfocused should strip to the same layout
	for _, focused := range []bool{true, false} {
		resultA := setBorderBottomCounts(rendered, 42, 13, focused)
		lastNl := strings.LastIndexByte(resultA, '\n')
		require.NotEqual(t, -1, lastNl)
		bottomA := resultA[lastNl+1:]
		plainA := stripANSI(bottomA)

		// The stripped width should match the original border width
		assert.Equal(t, paneWidth, ansi.StringWidth(bottomA),
			"ANSI-aware width mismatch (focused=%v)", focused)
		assert.Equal(t, paneWidth, len([]rune(plainA)),
			"plain rune count mismatch (focused=%v)", focused)
	}
}

func TestSetBorderBottomCounts_NarrowPaneFallback(t *testing.T) {
	// For very narrow panes where leftFill or rightFill would be negative,
	// the function should fall back without panicking.
	rendered := makePlainBottomBorder(10)
	result := setBorderBottomCounts(rendered, 99999, 99999, false)
	// Should not panic and should produce a valid string
	assert.NotEmpty(t, result)
}

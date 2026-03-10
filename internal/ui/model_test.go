package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justincampbell/revise/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeModel(paths ...string) Model {
	files := make([]git.FileDiff, len(paths))
	for i, p := range paths {
		files[i] = git.FileDiff{Path: p, Status: git.StatusModified}
	}
	m := New(&git.Diff{Files: files}, false)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(Model)
}

// makeModelWithDiff creates a model with a single file containing actual diff lines.
func makeModelWithDiff(filePath string, lines []git.Line) Model {
	hunk := git.Hunk{
		Header: "@@ -1,1 +1,1 @@",
		Lines:  lines,
	}
	files := []git.FileDiff{{
		Path:   filePath,
		Status: git.StatusModified,
		Hunks:  []git.Hunk{hunk},
	}}
	m := New(&git.Diff{Files: files}, false)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(Model)
}

func sendKey(m Model, key string) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return updated.(Model)
}

func sendSpecialKey(m Model, keyType tea.KeyType) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: keyType})
	return updated.(Model)
}

// focusDiffAndMoveToline focuses the diff view and moves the cursor n lines down.
func focusDiffAndMoveTo(m Model, n int) Model {
	m = sendKey(m, "l") // focus diff view
	for i := 0; i < n; i++ {
		m = sendSpecialKey(m, tea.KeyDown)
	}
	return m
}

func TestModelInitialFocus(t *testing.T) {
	m := makeModel("a.go", "b.go")
	assert.Equal(t, focusFileList, m.focus)
}

func TestModelRightKey_FocusesDiff(t *testing.T) {
	m := makeModel("a.go")
	m = sendSpecialKey(m, tea.KeyRight)
	assert.Equal(t, focusDiffView, m.focus)
}

func TestModelLKey_FocusesDiff(t *testing.T) {
	m := makeModel("a.go")
	m = sendKey(m, "l")
	assert.Equal(t, focusDiffView, m.focus)
}

func TestModelLeftKey_FocusesFileList(t *testing.T) {
	m := makeModel("a.go")
	m = sendSpecialKey(m, tea.KeyRight)
	m = sendSpecialKey(m, tea.KeyLeft)
	assert.Equal(t, focusFileList, m.focus)
}

func TestModelHKey_FocusesFileList(t *testing.T) {
	m := makeModel("a.go")
	m = sendKey(m, "l")
	m = sendKey(m, "h")
	assert.Equal(t, focusFileList, m.focus)
}

func TestModelEsc_FocusesFileList(t *testing.T) {
	m := makeModel("a.go")
	m = sendKey(m, "l")
	m = sendSpecialKey(m, tea.KeyEsc)
	assert.Equal(t, focusFileList, m.focus)
}

func TestModelEsc_ExitsFullscreenAndFocusesFileList(t *testing.T) {
	m := makeModel("a.go")
	m = sendKey(m, "f")
	require.True(t, m.fullscreen)
	m = sendSpecialKey(m, tea.KeyEsc)
	assert.False(t, m.fullscreen)
	assert.Equal(t, focusFileList, m.focus)
}

func TestModelF_TogglesFullscreen(t *testing.T) {
	m := makeModel("a.go")
	m = sendKey(m, "f")
	assert.True(t, m.fullscreen)
	m = sendKey(m, "f")
	assert.False(t, m.fullscreen)
}

func TestModelF_FocusesDiffWhenEnteringFullscreen(t *testing.T) {
	m := makeModel("a.go")
	m = sendKey(m, "f")
	assert.Equal(t, focusDiffView, m.focus)
}

func TestModelQuestionMark_TogglesHelp(t *testing.T) {
	m := makeModel("a.go")
	m = sendKey(m, "?")
	assert.True(t, m.showHelp)
	m = sendKey(m, "?")
	assert.False(t, m.showHelp)
}

func TestModelN_NextFile(t *testing.T) {
	m := makeModel("a.go", "b.go", "c.go")
	m = sendKey(m, "n")
	assert.Equal(t, 1, m.fileList.cursor)
}

func TestModelShiftN_PrevFile(t *testing.T) {
	m := makeModel("a.go", "b.go", "c.go")
	m = sendKey(m, "n")
	m = sendKey(m, "n")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	m = updated.(Model)
	assert.Equal(t, 1, m.fileList.cursor)
}

func TestModelLeftKey_ExitsFullscreen(t *testing.T) {
	m := makeModel("a.go")
	m = sendKey(m, "f")
	m = sendSpecialKey(m, tea.KeyLeft)
	assert.False(t, m.fullscreen)
	assert.Equal(t, focusFileList, m.focus)
}

func TestModelRightKey_WhenAlreadyFocused_TogglesFullscreen(t *testing.T) {
	m := makeModel("a.go")
	m = sendKey(m, "l") // focus diff
	assert.Equal(t, focusDiffView, m.focus)
	assert.False(t, m.fullscreen)
	m = sendKey(m, "l") // right again → fullscreen
	assert.True(t, m.fullscreen)
	m = sendKey(m, "l") // right again → exit fullscreen
	assert.False(t, m.fullscreen)
}

func TestModelRightKey_WhenFileListFocused_FocusesDiff(t *testing.T) {
	m := makeModel("a.go")
	assert.Equal(t, focusFileList, m.focus)
	m = sendKey(m, "l")
	assert.Equal(t, focusDiffView, m.focus)
	assert.False(t, m.fullscreen)
}

func TestModelHelpDismissedByAnyKey(t *testing.T) {
	m := makeModel("a.go")
	m = sendKey(m, "?")
	require.True(t, m.showHelp)
	m = sendKey(m, "x")
	assert.False(t, m.showHelp)
}

// --- Comment tests ---

func TestModelComment_CursorStartsOnFirstCodeLine(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	// After init, cursor should be on the first code line, not the file header.
	assert.NotNil(t, m.diffView.cursorRef())
}

func TestModelComment_PressC_OnCodeLine_EntersCommentMode(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	// lines: [0: header, 1: blank, 2: hunk, 3: code, 4: blank]
	m = focusDiffAndMoveTo(m, 0)
	require.NotNil(t, m.diffView.cursorRef())
	m = sendKey(m, "c")
	assert.True(t, m.commentInputActive)
}

func TestModelComment_InputEsc_Cancels(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m = focusDiffAndMoveTo(m, 0)
	m = sendKey(m, "c")
	require.True(t, m.commentInputActive)
	m = sendSpecialKey(m, tea.KeyEsc)
	assert.False(t, m.commentInputActive)
}

func TestModelComment_TypeAndSave(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m = focusDiffAndMoveTo(m, 0)
	m = sendKey(m, "c")
	m = sendKey(m, "h")
	m = sendKey(m, "i")
	assert.Equal(t, "hi", m.diffView.textInput.Value())
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	assert.False(t, m.commentInputActive)
	assert.Equal(t, "hi", m.comments[commentKey{file: "foo.go", lineNum: 1}])
}

func TestModelComment_DeleteComment(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	// Add a comment directly
	m.comments[commentKey{file: "foo.go", lineNum: 1}] = "to delete"
	m = focusDiffAndMoveTo(m, 0)
	m = sendKey(m, "d")
	_, exists := m.comments[commentKey{file: "foo.go", lineNum: 1}]
	assert.False(t, exists)
}

func TestModelComment_EditExistingComment_PreFills(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m.comments[commentKey{file: "foo.go", lineNum: 1}] = "existing"
	m = focusDiffAndMoveTo(m, 0)
	m = sendKey(m, "c")
	assert.True(t, m.commentInputActive)
	assert.Equal(t, "existing", m.diffView.textInput.Value())
}

func TestModelComment_Backspace(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m = focusDiffAndMoveTo(m, 0)
	m = sendKey(m, "c")
	m = sendKey(m, "a")
	m = sendKey(m, "b")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(Model)
	assert.Equal(t, "a", m.diffView.textInput.Value())
}

func TestModelComment_ClearInputOnEnter_DeletesComment(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m.comments[commentKey{file: "foo.go", lineNum: 1}] = "hi"
	m = focusDiffAndMoveTo(m, 0)
	m = sendKey(m, "c")
	require.Equal(t, "hi", m.diffView.textInput.Value()) // pre-filled
	// Clear the input with backspace, then press enter
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(Model)
	require.Empty(t, m.diffView.textInput.Value())
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	_, exists := m.comments[commentKey{file: "foo.go", lineNum: 1}]
	assert.False(t, exists)
}

func TestModelEnter_OnCodeLine_EntersCommentMode(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m = focusDiffAndMoveTo(m, 0)
	require.NotNil(t, m.diffView.cursorRef())
	m = sendSpecialKey(m, tea.KeyEnter)
	assert.True(t, m.commentInputActive)
}

func TestModelExport_WorksFromFileListFocus(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m.comments[commentKey{file: "foo.go", lineNum: 1}] = "a comment"
	require.Equal(t, focusFileList, m.focus)
	m = sendKey(m, "e")
	// Should have set a status message (export happened)
	assert.NotEmpty(t, m.statusMsg)
}

func TestModelExport_SetsStatusMessage(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m.comments[commentKey{file: "foo.go", lineNum: 1}] = "a comment"
	m = sendKey(m, "e")
	assert.NotEmpty(t, m.statusMsg)
}

// --- DiffMode cycling tests ---

func TestCycleMode_ForwardOnFeatureBranch(t *testing.T) {
	m := makeModel("a.go")
	// Default mode on feature branch is ModeBranch (broadest)
	assert.Equal(t, ModeBranch, m.mode)
	m.cycleMode(+1)
	assert.Equal(t, ModeStaged, m.mode)
	m.cycleMode(+1)
	assert.Equal(t, ModeUnstaged, m.mode)
	m.cycleMode(+1)
	assert.Equal(t, ModeBranch, m.mode) // wraps
}

func TestCycleMode_BackwardOnFeatureBranch(t *testing.T) {
	m := makeModel("a.go")
	m.cycleMode(-1)
	assert.Equal(t, ModeUnstaged, m.mode) // wraps backward
	m.cycleMode(-1)
	assert.Equal(t, ModeStaged, m.mode)
}

func TestCycleMode_SkipsBranchOnDefaultBranch(t *testing.T) {
	m := New(&git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}}, true)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	assert.Equal(t, ModeStaged, m.mode) // starts on broadest available
	m.cycleMode(+1)
	assert.Equal(t, ModeUnstaged, m.mode)
	m.cycleMode(+1)
	assert.Equal(t, ModeStaged, m.mode) // wraps, skips Branch
}

func TestCycleMode_BackwardSkipsBranchOnDefaultBranch(t *testing.T) {
	m := New(&git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}}, true)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	m.cycleMode(-1)
	assert.Equal(t, ModeUnstaged, m.mode) // wraps backward, skips Branch
}

func TestNewModel_FeatureBranch_StartsOnBranch(t *testing.T) {
	m := New(&git.Diff{}, false)
	assert.Equal(t, ModeBranch, m.mode)
}

func TestNewModel_DefaultBranch_StartsOnStaged(t *testing.T) {
	m := New(&git.Diff{}, true)
	assert.Equal(t, ModeStaged, m.mode)
}

// --- Mode slider rendering tests ---

func TestModeSlider_FeatureBranch_BranchMode_AllLit(t *testing.T) {
	m := makeModel("a.go") // feature branch, starts on ModeBranch
	slider := m.renderModeSlider()
	// All three labels should appear
	assert.Contains(t, slider, "Branch")
	assert.Contains(t, slider, "Staged")
	assert.Contains(t, slider, "Unstaged")
	assert.Contains(t, slider, "Tab: switch")
}

func TestModeSlider_ShowsContextLineCount(t *testing.T) {
	m := makeModel("a.go")
	slider := m.renderModeSlider()
	assert.Contains(t, slider, "+/-: context (3)")
}

func TestModeSlider_DefaultBranch_OmitsBranch(t *testing.T) {
	m := New(&git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}}, true)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	slider := m.renderModeSlider()
	assert.NotContains(t, slider, "Branch")
	assert.Contains(t, slider, "Staged")
	assert.Contains(t, slider, "Unstaged")
}

func TestModelExport_NoCommentsNoStatus(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m = sendKey(m, "e")
	assert.Empty(t, m.statusMsg)
}

func TestModelComment_CommentInputBlocksOtherKeys(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m = focusDiffAndMoveTo(m, 0)
	m = sendKey(m, "c")
	require.True(t, m.commentInputActive)
	// Pressing "q" should add to input, not quit
	m = sendKey(m, "q")
	assert.True(t, m.commentInputActive)
	assert.Equal(t, "q", m.diffView.textInput.Value())
}

// --- Hunk navigation tests ---

func TestModelNextHunk_Key(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineContext, Content: "a", OldNum: 1, NewNum: 1},
		{Type: git.LineAdded, Content: "b", NewNum: 2},
	})
	m = sendKey(m, "l") // focus diff
	startCursor := m.diffView.cursor
	// } should move to next hunk (or stay if only one)
	m = sendKey(m, "}")
	// With single hunk, cursor stays same
	assert.Equal(t, startCursor, m.diffView.cursor)
}

func TestModelPrevHunk_Key(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineContext, Content: "a", OldNum: 1, NewNum: 1},
		{Type: git.LineAdded, Content: "b", NewNum: 2},
	})
	m = sendKey(m, "l") // focus diff
	m = sendKey(m, "{")
	// With single hunk at top, cursor stays at first navigable line
	assert.NotNil(t, m.diffView.cursorRef())
}

// --- Context lines tests ---

func TestModelPlus_IncreasesContextLines(t *testing.T) {
	m := makeModel("a.go")
	assert.Equal(t, git.DefaultContextLines, m.contextLines)
	m = sendKey(m, "+")
	assert.Equal(t, git.DefaultContextLines+1, m.contextLines)
}

func TestModelMinus_DecreasesContextLines(t *testing.T) {
	m := makeModel("a.go")
	m.contextLines = 5
	m = sendKey(m, "-")
	assert.Equal(t, 4, m.contextLines)
}

func TestModelMinus_ClampsAtZero(t *testing.T) {
	m := makeModel("a.go")
	m.contextLines = 0
	m = sendKey(m, "-")
	assert.Equal(t, 0, m.contextLines)
}

func TestModelEquals_IncreasesContextLines(t *testing.T) {
	// "=" is the unshifted version of "+" on most keyboards
	m := makeModel("a.go")
	m = sendKey(m, "=")
	assert.Equal(t, git.DefaultContextLines+1, m.contextLines)
}

func TestModelUnderscore_DecreasesContextLines(t *testing.T) {
	// "_" is the shifted version of "-" on most keyboards
	m := makeModel("a.go")
	m.contextLines = 5
	m = sendKey(m, "_")
	assert.Equal(t, 4, m.contextLines)
}

func TestRenderStatusBar_FileListNoPaneTotals(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "a", NewNum: 1},
		{Type: git.LineRemoved, Content: "b", OldNum: 1},
		{Type: git.LineAdded, Content: "c", NewNum: 2},
	})
	m.focus = focusFileList
	status := m.renderStatusBar()
	assert.NotContains(t, status, "+2/-1")
}

func TestRenderStatusBar_DiffNoPaneTotals(t *testing.T) {
	files := []git.FileDiff{
		{
			Path:   "a.go",
			Status: git.StatusModified,
			Hunks: []git.Hunk{{
				Header: "@@ -1,1 +1,2 @@",
				Lines: []git.Line{
					{Type: git.LineAdded, Content: "a", NewNum: 1},
					{Type: git.LineRemoved, Content: "b", OldNum: 1},
				},
			}},
		},
		{
			Path:   "b.go",
			Status: git.StatusModified,
			Hunks: []git.Hunk{{
				Header: "@@ -1,0 +1,1 @@",
				Lines: []git.Line{
					{Type: git.LineAdded, Content: "c", NewNum: 1},
				},
			}},
		},
	}
	m := New(&git.Diff{Files: files}, false)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	m.focus = focusDiffView
	status := m.renderStatusBar()
	assert.NotContains(t, status, "a.go +1/-1")
}

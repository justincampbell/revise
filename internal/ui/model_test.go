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

func TestModeSlider_BranchMode_PlusDelimiters(t *testing.T) {
	m := makeModel("a.go") // feature branch, ModeBranch
	slider := m.renderModeSlider()
	// All active — should use + between all labels
	assert.Contains(t, slider, "+")
	assert.NotContains(t, slider, "·")
}

func TestModeSlider_StagedMode_MixedDelimiters(t *testing.T) {
	m := makeModel("a.go")
	m.mode = ModeStaged
	slider := m.renderModeSlider()
	// Branch inactive, Staged+Unstaged active — + between Staged and Unstaged, space before Staged
	assert.Contains(t, slider, "+")
}

func TestModeSlider_UnstagedMode_SpaceDelimiters(t *testing.T) {
	m := makeModel("a.go")
	m.mode = ModeUnstaged
	slider := m.renderModeSlider()
	// Only Unstaged active — space delimiters between labels, no +
	assert.Contains(t, slider, "Branch Staged Unstaged")
}

// --- Slider click tests ---

func TestSliderModeAt_FeatureBranch_ClickBranch(t *testing.T) {
	m := makeModel("a.go")
	// "Branch·Staged·Unstaged" — "Branch" at positions 0-5
	assert.Equal(t, ModeBranch, m.sliderModeAt(0))
	assert.Equal(t, ModeBranch, m.sliderModeAt(5))
}

func TestSliderModeAt_FeatureBranch_ClickStaged(t *testing.T) {
	m := makeModel("a.go")
	// "Branch·Staged·Unstaged" — "Staged" at positions 7-12
	assert.Equal(t, ModeStaged, m.sliderModeAt(7))
	assert.Equal(t, ModeStaged, m.sliderModeAt(12))
}

func TestSliderModeAt_FeatureBranch_ClickUnstaged(t *testing.T) {
	m := makeModel("a.go")
	// "Branch·Staged·Unstaged" — "Unstaged" at positions 14-21
	assert.Equal(t, ModeUnstaged, m.sliderModeAt(14))
	assert.Equal(t, ModeUnstaged, m.sliderModeAt(21))
}

func TestSliderModeAt_FeatureBranch_ClickSeparator(t *testing.T) {
	m := makeModel("a.go")
	// "·" separator at position 6 — should return -1
	assert.Equal(t, DiffMode(-1), m.sliderModeAt(6))
}

func TestSliderModeAt_DefaultBranch_ClickStaged(t *testing.T) {
	m := New(&git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}}, true)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	// "Staged·Unstaged" — "Staged" at positions 0-5
	assert.Equal(t, ModeStaged, m.sliderModeAt(0))
	assert.Equal(t, ModeStaged, m.sliderModeAt(5))
}

func TestSliderModeAt_DefaultBranch_ClickUnstaged(t *testing.T) {
	m := New(&git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}}, true)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	// "Staged·Unstaged" — "Unstaged" at positions 7-14
	assert.Equal(t, ModeUnstaged, m.sliderModeAt(7))
	assert.Equal(t, ModeUnstaged, m.sliderModeAt(14))
}

func TestSliderModeAt_OutsideLabels(t *testing.T) {
	m := makeModel("a.go")
	assert.Equal(t, DiffMode(-1), m.sliderModeAt(50))
	assert.Equal(t, DiffMode(-1), m.sliderModeAt(-1))
}

func TestMouseClickSlider_SwitchesMode(t *testing.T) {
	m := makeModel("a.go")
	assert.Equal(t, ModeBranch, m.mode)

	// Click on "Unstaged" label (position 14) in the status bar row
	mouseMsg := tea.MouseMsg{
		X:      14,
		Y:      m.height - 1,
		Button: tea.MouseButtonLeft,
	}
	updated, cmd := m.Update(mouseMsg)
	m = updated.(Model)
	assert.Equal(t, ModeUnstaged, m.mode)
	assert.NotNil(t, cmd, "should return a command to load the diff")
}

func TestMouseClickSlider_SameMode_NoReload(t *testing.T) {
	m := makeModel("a.go")
	assert.Equal(t, ModeBranch, m.mode)

	// Click on "Branch" label (position 0) — already active
	mouseMsg := tea.MouseMsg{
		X:      0,
		Y:      m.height - 1,
		Button: tea.MouseButtonLeft,
	}
	updated, cmd := m.Update(mouseMsg)
	m = updated.(Model)
	assert.Equal(t, ModeBranch, m.mode)
	assert.Nil(t, cmd, "should not reload when clicking the already-active mode")
}

func TestModelExport_NoCommentsNoStatus(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m = sendKey(m, "e")
	assert.Empty(t, m.statusMsg)
}

func TestModelReportIssue_SetsStatusAndReturnsCmd(t *testing.T) {
	m := makeModel("a.go")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
	m = updated.(Model)
	assert.Contains(t, m.statusMsg, "Opening")
	assert.NotNil(t, cmd, "should return a command to open the URL")
}

func TestModelReportIssue_WorksFromDiffView(t *testing.T) {
	m := makeModel("a.go")
	m = sendKey(m, "l") // focus diff
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
	m = updated.(Model)
	assert.Contains(t, m.statusMsg, "Opening")
	assert.NotNil(t, cmd, "should return a command to open the URL")
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

// --- Stage/Unstage/Discard tests ---

// makeModelWithSourcedHunk creates a model with a single hunk from the given source.
func makeModelWithSourcedHunk(source git.HunkSource) Model {
	hunk := git.Hunk{
		Header:   "@@ -1,3 +1,4 @@",
		Source:   source,
		OldStart: 1, OldCount: 3, NewStart: 1, NewCount: 4,
		Lines: []git.Line{
			{Type: git.LineContext, Content: "a", OldNum: 1, NewNum: 1},
			{Type: git.LineAdded, Content: "b", NewNum: 2},
			{Type: git.LineContext, Content: "c", OldNum: 2, NewNum: 3},
			{Type: git.LineContext, Content: "d", OldNum: 3, NewNum: 4},
		},
	}
	files := []git.FileDiff{{
		Path:   "test.go",
		Status: git.StatusModified,
		Hunks:  []git.Hunk{hunk},
	}}
	m := New(&git.Diff{Files: files}, false)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(Model)
}

func TestModelStageKey_OnUnstagedHunk_ReturnsCmd(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceUnstaged)
	m = sendKey(m, "l") // focus diff
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updated.(Model)
	assert.NotNil(t, cmd, "should return a command for staging")
	assert.Empty(t, m.statusMsg)
}

func TestModelStageKey_OnStagedHunk_ShowsError(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceStaged)
	m = sendKey(m, "l") // focus diff
	m = sendKey(m, "s")
	assert.Contains(t, m.statusMsg, "unstaged")
}

func TestModelStageKey_OnBranchHunk_ShowsError(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceBranch)
	m = sendKey(m, "l") // focus diff
	m = sendKey(m, "s")
	assert.Contains(t, m.statusMsg, "unstaged")
}

func TestModelUnstageKey_OnStagedHunk_ReturnsCmd(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceStaged)
	m = sendKey(m, "l") // focus diff
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	m = updated.(Model)
	assert.NotNil(t, cmd, "should return a command for unstaging")
	assert.Empty(t, m.statusMsg)
}

func TestModelUnstageKey_OnUnstagedHunk_ShowsError(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceUnstaged)
	m = sendKey(m, "l") // focus diff
	m = sendKey(m, "u")
	assert.Contains(t, m.statusMsg, "staged")
}

func TestModelDiscardKey_OnUnstagedHunk_PromptsConfirmation(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceUnstaged)
	m = sendKey(m, "l") // focus diff
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	m = updated.(Model)
	assert.True(t, m.confirmDiscard)
	assert.Contains(t, m.confirmMsg, "cannot be undone")
}

func TestModelDiscardKey_ConfirmWithY_ReturnsCmd(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceUnstaged)
	m = sendKey(m, "l") // focus diff
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	m = updated.(Model)
	require.True(t, m.confirmDiscard)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = updated.(Model)
	assert.False(t, m.confirmDiscard)
	assert.NotNil(t, cmd, "should return a command after confirmation")
}

func TestModelDiscardKey_ConfirmWithEnter(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceUnstaged)
	m = sendKey(m, "l") // focus diff
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	m = updated.(Model)
	require.True(t, m.confirmDiscard)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	assert.False(t, m.confirmDiscard)
	assert.NotNil(t, cmd, "Enter should confirm discard")
}

func TestModelDiscardKey_CancelWithOtherKey(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceUnstaged)
	m = sendKey(m, "l") // focus diff
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	m = updated.(Model)
	require.True(t, m.confirmDiscard)
	m = sendKey(m, "n")
	assert.False(t, m.confirmDiscard)
	assert.Empty(t, m.confirmMsg)
}

func TestModelDiscardKey_OnStagedHunk_PromptsConfirmation(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceStaged)
	m = sendKey(m, "l") // focus diff
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	m = updated.(Model)
	assert.True(t, m.confirmDiscard)
	assert.Contains(t, m.confirmMsg, "cannot be undone")
}

func TestModelDiscardKey_OnBranchHunk_ShowsError(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceBranch)
	m = sendKey(m, "l") // focus diff
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	m = updated.(Model)
	assert.Contains(t, m.statusMsg, "committed")
	assert.False(t, m.confirmDiscard)
}

func TestModelStageFileKey_WithUnstagedHunks_ReturnsCmd(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceUnstaged)
	m = sendKey(m, "l") // focus diff
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	m = updated.(Model)
	assert.NotNil(t, cmd, "should return a command for staging file")
	assert.Empty(t, m.statusMsg)
}

func TestModelStageFileKey_NoUnstagedHunks_ShowsError(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceStaged)
	m = sendKey(m, "l") // focus diff
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	m = updated.(Model)
	assert.Contains(t, m.statusMsg, "No unstaged")
}

func TestModelStageFileKey_BranchHunksOnly_ShowsError(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceBranch)
	m = sendKey(m, "l") // focus diff
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	m = updated.(Model)
	assert.Contains(t, m.statusMsg, "No unstaged")
}

func TestModelUnstageFileKey_WithStagedHunks_ReturnsCmd(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceStaged)
	m = sendKey(m, "l") // focus diff
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("U")})
	m = updated.(Model)
	assert.NotNil(t, cmd, "should return a command for unstaging file")
	assert.Empty(t, m.statusMsg)
}

func TestModelUnstageFileKey_NoStagedHunks_ShowsError(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceUnstaged)
	m = sendKey(m, "l") // focus diff
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("U")})
	m = updated.(Model)
	assert.Contains(t, m.statusMsg, "No staged")
}

// --- File list stage/unstage tests ---

func TestModelFileList_S_StagesFile(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceUnstaged)
	require.Equal(t, focusFileList, m.focus)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updated.(Model)
	assert.NotNil(t, cmd, "should return a command for staging file")
	assert.Empty(t, m.statusMsg)
}

func TestModelFileList_S_NoUnstaged_ShowsError(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceStaged)
	require.Equal(t, focusFileList, m.focus)
	m = sendKey(m, "s")
	assert.Contains(t, m.statusMsg, "No unstaged")
}

func TestModelFileList_ShiftS_StagesFile(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceUnstaged)
	require.Equal(t, focusFileList, m.focus)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	m = updated.(Model)
	assert.NotNil(t, cmd, "S should stage file from file list")
	assert.Empty(t, m.statusMsg)
}

func TestModelFileList_U_UnstagesFile(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceStaged)
	require.Equal(t, focusFileList, m.focus)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	m = updated.(Model)
	assert.NotNil(t, cmd, "should return a command for unstaging file")
	assert.Empty(t, m.statusMsg)
}

func TestModelFileList_U_NoStaged_ShowsError(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceUnstaged)
	require.Equal(t, focusFileList, m.focus)
	m = sendKey(m, "u")
	assert.Contains(t, m.statusMsg, "No staged")
}

func TestModelFileList_ShiftU_UnstagesFile(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceStaged)
	require.Equal(t, focusFileList, m.focus)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("U")})
	m = updated.(Model)
	assert.NotNil(t, cmd, "U should unstage file from file list")
	assert.Empty(t, m.statusMsg)
}

func TestModelFileList_D_PromptsConfirmation(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceUnstaged)
	require.Equal(t, focusFileList, m.focus)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	m = updated.(Model)
	assert.True(t, m.confirmDiscard)
	assert.Contains(t, m.confirmMsg, "cannot be undone")
}

func TestModelFileList_D_ConfirmWithY_ReturnsCmd(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceUnstaged)
	require.Equal(t, focusFileList, m.focus)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	m = updated.(Model)
	require.True(t, m.confirmDiscard)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = updated.(Model)
	assert.False(t, m.confirmDiscard)
	assert.NotNil(t, cmd, "should return a command after confirmation")
}

func TestModelFileList_D_StagedFile_PromptsConfirmation(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceStaged)
	require.Equal(t, focusFileList, m.focus)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	m = updated.(Model)
	assert.True(t, m.confirmDiscard)
	assert.Contains(t, m.confirmMsg, "cannot be undone")
}

func TestModelFileList_D_BranchOnlyFile_ShowsError(t *testing.T) {
	m := makeModelWithSourcedHunk(git.SourceBranch)
	require.Equal(t, focusFileList, m.focus)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	m = updated.(Model)
	assert.Contains(t, m.statusMsg, "No changes")
	assert.False(t, m.confirmDiscard)
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

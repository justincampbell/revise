package ui

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/justincampbell/revise/internal/git"
	"github.com/justincampbell/revise/internal/update"
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

func TestModelRightKey_WhenAlreadyFocused_DoesNotToggleFullscreen(t *testing.T) {
	m := makeModel("a.go")
	m = sendKey(m, "l") // focus diff
	require.Equal(t, focusDiffView, m.focus)
	require.False(t, m.fullscreen)
	m = sendKey(m, "l") // right again → must NOT toggle fullscreen
	assert.False(t, m.fullscreen, "right arrow must not toggle fullscreen")
	m = sendKey(m, "f") // enter fullscreen explicitly
	require.True(t, m.fullscreen)
	m = sendKey(m, "l") // right again → must NOT exit fullscreen
	assert.True(t, m.fullscreen, "right arrow must not exit fullscreen")
}

func TestModelRightKey_WhenFileListFocused_FocusesDiff(t *testing.T) {
	m := makeModel("a.go")
	assert.Equal(t, focusFileList, m.focus)
	m = sendKey(m, "l")
	assert.Equal(t, focusDiffView, m.focus)
	assert.False(t, m.fullscreen)
}

func TestModelRightKey_WhenDiffFocused_ScrollsRight(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: strings.Repeat("A", 500), NewNum: 1},
	})
	m = sendKey(m, "l") // focus diff
	require.Equal(t, focusDiffView, m.focus)
	require.Equal(t, 0, m.diffView.hOffset)
	m = sendKey(m, "l") // right again → scroll right
	assert.Greater(t, m.diffView.hOffset, 0, "right arrow should scroll right")
}

func TestModelLeftKey_WhenScrolledRight_ScrollsLeft(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: strings.Repeat("A", 500), NewNum: 1},
	})
	m = sendKey(m, "l")
	m = sendKey(m, "l")
	m = sendKey(m, "l")
	require.Greater(t, m.diffView.hOffset, 0)
	prev := m.diffView.hOffset
	m = sendKey(m, "h") // left while scrolled right → scroll left, not focus change
	assert.Less(t, m.diffView.hOffset, prev)
	assert.Equal(t, focusDiffView, m.focus, "left must not change focus while scrolled")
}

func TestModelLeftKey_AtZeroScroll_FocusesFileList(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "short", NewNum: 1},
	})
	m = sendKey(m, "l")
	require.Equal(t, focusDiffView, m.focus)
	require.Equal(t, 0, m.diffView.hOffset)
	m = sendKey(m, "h")
	assert.Equal(t, focusFileList, m.focus)
}

func TestModelMouseWheelRight_ScrollsDiffRight(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: strings.Repeat("A", 500), NewNum: 1},
	})
	m = sendKey(m, "l")
	require.Equal(t, 0, m.diffView.hOffset)
	// Click inside the diff pane (x past file list width + gap).
	updated, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelRight, X: 50, Y: 5})
	m = updated.(Model)
	assert.Greater(t, m.diffView.hOffset, 0)
}

func TestModelMouseWheelLeft_ScrollsDiffLeft(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: strings.Repeat("A", 500), NewNum: 1},
	})
	m = sendKey(m, "l")
	m.diffView.hOffset = 20
	updated, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelLeft, X: 50, Y: 5})
	m = updated.(Model)
	assert.Less(t, m.diffView.hOffset, 20)
}

func TestModelHelpDismissedByAnyKey(t *testing.T) {
	m := makeModel("a.go")
	m = sendKey(m, "?")
	require.True(t, m.showHelp)
	m = sendKey(m, "x")
	assert.False(t, m.showHelp)
}

func TestModelHelpMouseScrollDown(t *testing.T) {
	m := makeModel("a.go")
	m = sendKey(m, "?")
	require.True(t, m.showHelp)
	updated, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	m = updated.(Model)
	assert.Equal(t, 1, m.helpScroll)
}

func TestModelHelpMouseScrollUp(t *testing.T) {
	m := makeModel("a.go")
	m = sendKey(m, "?")
	m.helpScroll = 5
	updated, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	m = updated.(Model)
	assert.Equal(t, 4, m.helpScroll)
}

func TestModelHelpMouseScrollUpAtZero(t *testing.T) {
	m := makeModel("a.go")
	m = sendKey(m, "?")
	require.Equal(t, 0, m.helpScroll)
	updated, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	m = updated.(Model)
	assert.Equal(t, 0, m.helpScroll)
}

func TestModelHelpOverlayShowsBackground(t *testing.T) {
	m := makeModel("a.go")
	m = sendKey(m, "?")
	require.True(t, m.showHelp)
	view := m.View()
	// The help overlay should contain help content
	assert.Contains(t, view, "Keyboard Shortcuts")
	// The background (file list) should still be visible around the overlay
	assert.Contains(t, view, "a.go")
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
	assert.Equal(t, ModeStagedOnly, m.mode)
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
	assert.Equal(t, ModeStagedOnly, m.mode)
	m.cycleMode(-1)
	assert.Equal(t, ModeStaged, m.mode)
}

func TestCycleMode_SkipsBranchOnDefaultBranch(t *testing.T) {
	m := New(&git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}}, true)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	assert.Equal(t, ModeStaged, m.mode) // starts on broadest available
	m.cycleMode(+1)
	assert.Equal(t, ModeStagedOnly, m.mode)
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

func TestDiffRefresh_UpdatesOnDefaultBranch(t *testing.T) {
	// Start on default branch — Branch mode is not available
	m := New(&git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}}, true)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	require.True(t, m.onDefaultBranch)
	require.Equal(t, ModeStaged, m.mode)
	require.Equal(t, []DiffMode{ModeStaged, ModeStagedOnly, ModeUnstaged}, m.availableModes())

	// Simulate a diff refresh that reports we're no longer on the default branch
	// (e.g., user ran `git checkout -b feature` while the app was running)
	updated, _ = m.Update(diffLoadedMsg{
		diff:            &git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}},
		onDefaultBranch: false,
	})
	m = updated.(Model)

	assert.False(t, m.onDefaultBranch)
	assert.Equal(t, []DiffMode{ModeBranch, ModeStaged, ModeStagedOnly, ModeUnstaged}, m.availableModes())
}

// Regression for #148: when the branch diverges from default while revise is
// running (e.g. user committed on a fresh branch), promote mode to ModeBranch
// so the new committed changes become visible — unless the user has explicitly
// chosen a different mode.
func TestDiffRefresh_SwitchesToFeatureBranch_PromotesToBranchMode(t *testing.T) {
	m := New(&git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}}, true)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	require.Equal(t, ModeStaged, m.mode)
	require.False(t, m.modeExplicitlySet)

	updated, _ = m.Update(diffLoadedMsg{
		diff:            &git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}},
		onDefaultBranch: false,
	})
	m = updated.(Model)

	assert.Equal(t, ModeBranch, m.mode, "should promote to ModeBranch when branch diverges and mode is the default")
}

// When mode auto-promotes to ModeBranch on the default→feature transition,
// the diffLoadedMsg being applied was loaded under the OLD mode (e.g.
// ModeStaged), so its contents are wrong for the new mode. The handler must
// return a follow-up reload command — otherwise the user sees Branch mode
// with empty contents until they manually refresh.
func TestDiffRefresh_AutoPromote_KicksOffReload(t *testing.T) {
	m := New(&git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}}, true)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	require.True(t, m.onDefaultBranch)
	require.Equal(t, ModeStaged, m.mode)

	updated, cmd := m.Update(diffLoadedMsg{
		diff:            &git.Diff{},
		onDefaultBranch: false,
		fromPoll:        true,
	})
	m = updated.(Model)
	require.Equal(t, ModeBranch, m.mode)
	require.NotNil(t, cmd, "should return a follow-up command to reload the diff under the new mode")
}

// If the user explicitly picked a mode on the default branch, respect that
// choice across the transition — don't auto-promote to ModeBranch.
func TestDiffRefresh_SwitchesToFeatureBranch_PreservesExplicitMode(t *testing.T) {
	m := New(&git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}}, true)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	// Simulate Tab/Shift+Tab — cycleMode marks mode as explicit.
	m.cycleMode(+1)
	require.Equal(t, ModeStagedOnly, m.mode)
	require.True(t, m.modeExplicitlySet)

	updated, _ = m.Update(diffLoadedMsg{
		diff:            &git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}},
		onDefaultBranch: false,
	})
	m = updated.(Model)

	assert.Equal(t, ModeStagedOnly, m.mode, "should preserve explicitly-chosen mode across branch divergence")
}

func TestDiffRefresh_SwitchesToDefaultBranch_ClampsMode(t *testing.T) {
	// Start on feature branch in ModeBranch
	m := makeModel("a.go")
	require.Equal(t, ModeBranch, m.mode)

	// Switch to default branch — ModeBranch is no longer available, should clamp to ModeStaged
	updated, _ := m.Update(diffLoadedMsg{
		diff:            &git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}},
		onDefaultBranch: true,
	})
	m = updated.(Model)

	assert.True(t, m.onDefaultBranch)
	assert.Equal(t, ModeStaged, m.mode, "should clamp to ModeStaged when Branch is no longer available")
}

// --- Mode slider rendering tests ---

func TestModeSlider_FeatureBranch_BranchMode_AllLit(t *testing.T) {
	m := makeModel("a.go") // feature branch, starts on ModeBranch
	slider := m.renderModeSlider()
	// All three labels should appear
	assert.Contains(t, slider, "Branch")
	assert.Contains(t, slider, "Staged")
	assert.Contains(t, slider, "Unstaged")
}

func TestModeSlider_DefaultBranch_ShowsBranchDisabled(t *testing.T) {
	m := New(&git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}}, true)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	slider := m.renderModeSlider()
	// Branch is always shown — greyed out / disabled when unavailable.
	assert.Contains(t, ansi.Strip(slider), "Branch")
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

func TestModeSlider_StagedOnlyMode_ShowsStagedActive(t *testing.T) {
	m := makeModel("a.go")
	m.mode = ModeStagedOnly
	slider := m.renderModeSlider()
	// Only Staged label should be active
	assert.Contains(t, slider, "Staged")
	assert.Contains(t, slider, "Unstaged")
}

func TestModeSlider_UnstagedMode_SpaceDelimiters(t *testing.T) {
	m := makeModel("a.go")
	m.mode = ModeUnstaged
	slider := m.renderModeSlider()
	// Only Unstaged active — space delimiters between labels, no +
	assert.Contains(t, ansi.Strip(slider), "Branch Staged Unstaged")
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
	// "Branch Staged Unstaged" — "Staged" at positions 7-12 (Branch is shown but disabled)
	assert.Equal(t, ModeStaged, m.sliderModeAt(7))
	assert.Equal(t, ModeStaged, m.sliderModeAt(12))
}

func TestSliderModeAt_DefaultBranch_ClickUnstaged(t *testing.T) {
	m := New(&git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}}, true)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	// "Branch Staged Unstaged" — "Unstaged" at positions 14-21
	assert.Equal(t, ModeUnstaged, m.sliderModeAt(14))
	assert.Equal(t, ModeUnstaged, m.sliderModeAt(21))
}

// Clicking the Branch label while on the default branch (where it's disabled)
// should be a no-op — mode unchanged, no diff reload.
func TestMouseClickSlider_DefaultBranch_BranchDisabled_NoOp(t *testing.T) {
	m := New(&git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}}, true)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	require.Equal(t, ModeStaged, m.mode)

	mouseMsg := tea.MouseMsg{
		X:      0, // "Branch" label
		Y:      m.height - 1,
		Button: tea.MouseButtonLeft,
	}
	updated, cmd := m.Update(mouseMsg)
	m = updated.(Model)
	assert.Equal(t, ModeStaged, m.mode, "click on disabled Branch should not change mode")
	assert.Nil(t, cmd, "click on disabled Branch should not reload")
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

func TestMouseRelease_Ignored(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	// Open comment input on the first line
	m = sendKey(m, "c")
	require.True(t, m.commentInputActive)

	// Simulate mouse release on the diff panel — should be ignored
	releaseMsg := tea.MouseMsg{
		X:      40,
		Y:      2,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	}
	updated, _ := m.Update(releaseMsg)
	m = updated.(Model)
	assert.True(t, m.commentInputActive, "mouse release should not close the comment input")
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

func TestModelHardRefresh_SetsStatusAndReturnsCmd(t *testing.T) {
	m := makeModel("a.go")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = updated.(Model)
	assert.Contains(t, m.statusMsg, "Refreshing")
	assert.NotNil(t, cmd, "should return a command to reload the diff")
}

func TestModelHardRefresh_NoOpInFileReviewMode(t *testing.T) {
	m := New(&git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}}, false)
	m.fileReviewMode = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = updated.(Model)
	assert.Empty(t, m.statusMsg)
	assert.Nil(t, cmd)
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
	// } should jump to last navigable line when no next hunk
	m = sendKey(m, "}")
	// With single hunk, cursor moves to the last navigable line
	assert.Equal(t, 2, m.diffView.lineRefs[m.diffView.cursor].newNum)
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

// --- Cursor preservation tests ---

func TestDiffReload_PreservesCursorWhenFileRemoved(t *testing.T) {
	m := makeModel("a.go", "b.go", "c.go")
	// Move cursor to b.go (index 1)
	m = sendKey(m, "j")
	assert.Equal(t, 1, m.fileList.cursor)
	assert.Equal(t, "b.go", m.fileList.selectedFile().Path)

	// Simulate a diff reload where b.go was discarded/removed
	updated, _ := m.Update(diffLoadedMsg{
		diff: &git.Diff{Files: []git.FileDiff{
			{Path: "a.go", Status: git.StatusModified},
			{Path: "c.go", Status: git.StatusModified},
		}},
	})
	m = updated.(Model)
	// Cursor should stay at index 1 (now c.go), not reset to 0
	assert.Equal(t, 1, m.fileList.cursor)
	assert.Equal(t, "c.go", m.fileList.selectedFile().Path)
}

func TestDiffReload_ClampsCursorWhenLastFileRemoved(t *testing.T) {
	m := makeModel("a.go", "b.go")
	// Move cursor to b.go (index 1)
	m = sendKey(m, "j")
	assert.Equal(t, 1, m.fileList.cursor)

	// Simulate a diff reload where b.go was removed, leaving only a.go
	updated, _ := m.Update(diffLoadedMsg{
		diff: &git.Diff{Files: []git.FileDiff{
			{Path: "a.go", Status: git.StatusModified},
		}},
	})
	m = updated.(Model)
	// Cursor should clamp to last file (index 0)
	assert.Equal(t, 0, m.fileList.cursor)
}

// --- Hide whitespace tests ---

func TestModelW_TogglesHideWhitespace(t *testing.T) {
	m := makeModel("a.go")
	assert.False(t, m.hideWhitespace)
	m = sendKey(m, "w")
	assert.True(t, m.hideWhitespace)
	m = sendKey(m, "w")
	assert.False(t, m.hideWhitespace)
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

func TestRenderStatusBar_NoCommitsHint(t *testing.T) {
	m := New(&git.Diff{}, true)
	m.noCommits = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	assert.Contains(t, m.renderStatusBar(), "No commits yet")
}

func TestRenderStatusBar_NoCommitsHintHiddenWhenCommitsExist(t *testing.T) {
	m := New(&git.Diff{}, true)
	m.noCommits = false
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	assert.NotContains(t, m.renderStatusBar(), "No commits yet")
}

// --- Comment persistence tests ---

// makeModelWithStore creates a model with diff lines and a temp store path.
func makeModelWithStore(t *testing.T, filePath string, lines []git.Line) (Model, string) {
	t.Helper()
	storePath := filepath.Join(t.TempDir(), "comments.json")
	hunk := git.Hunk{
		Header: "@@ -1,1 +1,1 @@",
		Lines:  lines,
	}
	files := []git.FileDiff{{
		Path:   filePath,
		Status: git.StatusModified,
		Hunks:  []git.Hunk{hunk},
	}}
	m := NewWithStorePath(&git.Diff{Files: files}, false, storePath, "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(Model), storePath
}

func TestModelComment_PersistsOnSave(t *testing.T) {
	lines := []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	}
	m, storePath := makeModelWithStore(t, "foo.go", lines)

	// Add a comment
	m = focusDiffAndMoveTo(m, 0)
	m = sendKey(m, "c")
	m = sendKey(m, "h")
	m = sendKey(m, "i")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = updated.(Model)

	// Create a new model from the same store — should load the comment
	m2 := NewWithStorePath(&git.Diff{Files: []git.FileDiff{{
		Path:   "foo.go",
		Status: git.StatusModified,
		Hunks:  []git.Hunk{{Header: "@@ -1,1 +1,1 @@", Lines: lines}},
	}}}, false, storePath, "")
	updated2, _ := m2.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 = updated2.(Model)

	assert.Equal(t, "hi", m2.comments[commentKey{file: "foo.go", lineNum: 1}])
}

func TestModelComment_PersistsOnDelete(t *testing.T) {
	lines := []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	}
	m, storePath := makeModelWithStore(t, "foo.go", lines)

	// Add a comment
	m.comments[commentKey{file: "foo.go", lineNum: 1}] = "to delete"
	m.saveComments()

	// Delete the comment
	m = focusDiffAndMoveTo(m, 0)
	m = sendKey(m, "d")

	// Create a new model — comment should be gone
	m2 := NewWithStorePath(&git.Diff{Files: []git.FileDiff{{
		Path:   "foo.go",
		Status: git.StatusModified,
		Hunks:  []git.Hunk{{Header: "@@ -1,1 +1,1 @@", Lines: lines}},
	}}}, false, storePath, "")
	assert.Empty(t, m2.comments)
}

// --- Clear all comments tests ---

func TestModelClearComments_C_WithComments_PromptsConfirmation(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m.comments[commentKey{file: "foo.go", lineNum: 1}] = "a comment"
	m = sendKey(m, "C")
	assert.True(t, m.confirmClear)
	assert.Contains(t, m.confirmMsg, "1 item")
}

func TestModelClearComments_C_WithNoComments_DoesNothing(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m = sendKey(m, "C")
	assert.False(t, m.confirmClear)
}

func TestModelClearComments_ConfirmWithY_ClearsComments(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m.comments[commentKey{file: "foo.go", lineNum: 1}] = "a comment"
	m.comments[commentKey{file: "foo.go", lineNum: 2}] = "another"
	m = sendKey(m, "C")
	require.True(t, m.confirmClear)
	m = sendKey(m, "y")
	assert.False(t, m.confirmClear)
	assert.Empty(t, m.comments)
}

func TestModelClearComments_ConfirmWithEnter_ClearsComments(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m.comments[commentKey{file: "foo.go", lineNum: 1}] = "a comment"
	m = sendKey(m, "C")
	require.True(t, m.confirmClear)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	assert.False(t, m.confirmClear)
	assert.Empty(t, m.comments)
}

func TestModelClearComments_CancelWithOtherKey(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m.comments[commentKey{file: "foo.go", lineNum: 1}] = "a comment"
	m = sendKey(m, "C")
	require.True(t, m.confirmClear)
	m = sendKey(m, "n")
	assert.False(t, m.confirmClear)
	assert.Len(t, m.comments, 1)
}

func TestModelClearComments_PluralMessage(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m.comments[commentKey{file: "foo.go", lineNum: 1}] = "a"
	m.comments[commentKey{file: "foo.go", lineNum: 2}] = "b"
	m = sendKey(m, "C")
	assert.Contains(t, m.confirmMsg, "2 items")
}

func TestModelClearComments_Persists(t *testing.T) {
	lines := []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	}
	m, storePath := makeModelWithStore(t, "foo.go", lines)
	m.comments[commentKey{file: "foo.go", lineNum: 1}] = "a comment"
	m.saveComments()

	m = sendKey(m, "C")
	require.True(t, m.confirmClear)
	m = sendKey(m, "y")

	// Reload from store — should be empty
	m2 := NewWithStorePath(&git.Diff{Files: []git.FileDiff{{
		Path:   "foo.go",
		Status: git.StatusModified,
		Hunks:  []git.Hunk{{Header: "@@ -1,1 +1,1 @@", Lines: lines}},
	}}}, false, storePath, "")
	assert.Empty(t, m2.comments)
}

func TestModelClearComments_WorksFromDiffView(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m.comments[commentKey{file: "foo.go", lineNum: 1}] = "a comment"
	m = sendKey(m, "l") // focus diff
	m = sendKey(m, "C")
	assert.True(t, m.confirmClear)
}

func TestModelComment_NoStorePathDoesNotPersist(t *testing.T) {
	// With empty storePath, comments should still work in memory but not persist
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m = focusDiffAndMoveTo(m, 0)
	m = sendKey(m, "c")
	m = sendKey(m, "h")
	m = sendKey(m, "i")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	assert.Equal(t, "hi", m.comments[commentKey{file: "foo.go", lineNum: 1}])
	// No panic, no error — just works in memory
}

// --- File-level comment tests ---

func TestModelFileList_C_EntersCommentMode(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	require.Equal(t, focusFileList, m.focus)
	m = sendKey(m, "c")
	assert.True(t, m.commentInputActive)
	assert.Equal(t, commentKey{file: "foo.go", lineNum: 0}, m.commentTarget)
}

func TestModelFileList_C_TypeAndSave(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	require.Equal(t, focusFileList, m.focus)
	m = sendKey(m, "c")
	m = sendKey(m, "h")
	m = sendKey(m, "i")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	assert.False(t, m.commentInputActive)
	assert.Equal(t, "hi", m.comments[commentKey{file: "foo.go", lineNum: 0}])
}

func TestModelFileList_C_PreFillsExisting(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m.comments[commentKey{file: "foo.go", lineNum: 0}] = "existing"
	require.Equal(t, focusFileList, m.focus)
	m = sendKey(m, "c")
	assert.True(t, m.commentInputActive)
	assert.Equal(t, "existing", m.diffView.textInput.Value())
}

func TestModelFileList_C_NoFiles_DoesNothing(t *testing.T) {
	m := New(&git.Diff{Files: nil}, false)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	m = sendKey(m, "c")
	assert.False(t, m.commentInputActive)
}

func TestModelFileList_D_DeletesFileComment(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m.comments[commentKey{file: "foo.go", lineNum: 0}] = "to delete"
	require.Equal(t, focusFileList, m.focus)
	m = sendKey(m, "d")
	_, exists := m.comments[commentKey{file: "foo.go", lineNum: 0}]
	assert.False(t, exists)
}

func TestModelFileList_D_NoComment_DoesNothing(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	require.Equal(t, focusFileList, m.focus)
	// Should not panic or set status
	m = sendKey(m, "d")
	assert.Empty(t, m.statusMsg)
}

func TestModelFileList_C_Esc_Cancels(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m = sendKey(m, "c")
	require.True(t, m.commentInputActive)
	m = sendSpecialKey(m, tea.KeyEsc)
	assert.False(t, m.commentInputActive)
}

func TestModelFileComment_Persists(t *testing.T) {
	lines := []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	}
	m, storePath := makeModelWithStore(t, "foo.go", lines)

	// Add a file-level comment from file list
	require.Equal(t, focusFileList, m.focus)
	m = sendKey(m, "c")
	m = sendKey(m, "h")
	m = sendKey(m, "i")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// Create a new model from the same store — should load the file comment
	m2 := NewWithStorePath(&git.Diff{Files: []git.FileDiff{{
		Path:   "foo.go",
		Status: git.StatusModified,
		Hunks:  []git.Hunk{{Header: "@@ -1,1 +1,1 @@", Lines: lines}},
	}}}, false, storePath, "")
	updated2, _ := m2.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 = updated2.(Model)

	assert.Equal(t, "hi", m2.comments[commentKey{file: "foo.go", lineNum: 0}])
}

func TestInit_ReturnsPollTickCmd(t *testing.T) {
	m := makeModel("a.go")
	cmd := m.Init()
	assert.NotNil(t, cmd, "Init should return a poll tick command")
}

func TestUpdateCheckMsg_SetsStatusAndClearsAfterTimeout(t *testing.T) {
	m := makeModel("a.go")

	updated, cmd := m.Update(updateCheckMsg{
		info: &update.UpdateInfo{
			CurrentVersion: "0.1.0",
			LatestVersion:  "0.2.0",
			IsNewer:        true,
		},
	})
	m = updated.(Model)

	assert.Contains(t, m.statusMsg, "Update available")
	assert.NotNil(t, cmd, "should return a clear timer command")
}

func TestUpdateApplied_SetsStatusAndClearsAfterTimeout(t *testing.T) {
	m := makeModel("a.go")

	updated, cmd := m.Update(updateAppliedMsg{newVersion: "0.2.0"})
	m = updated.(Model)

	assert.Contains(t, m.statusMsg, "Updated to 0.2.0")
	assert.NotNil(t, cmd, "should return a clear timer command")
}

func TestRefreshResult_SameFingerprint_NoReload(t *testing.T) {
	m := makeModel("a.go")
	m.lastFingerprint = "M a.go\n"

	updated, cmd := m.Update(refreshResultMsg{fingerprint: "M a.go\n", duration: 100 * time.Millisecond})
	m = updated.(Model)
	assert.NotNil(t, cmd, "same fingerprint should still schedule next tick")
}

func TestRefreshResult_DifferentFingerprint_TriggersReload(t *testing.T) {
	m := makeModel("a.go")
	m.lastFingerprint = "M a.go\n"

	updated, cmd := m.Update(refreshResultMsg{fingerprint: "M a.go\n?? new.go\n", duration: 100 * time.Millisecond})
	m = updated.(Model)
	assert.NotNil(t, cmd, "different fingerprint should trigger a reload")
	assert.Equal(t, "M a.go\n?? new.go\n", m.lastFingerprint)
}

func TestPollRefresh_PreservesCursorPosition(t *testing.T) {
	lines := []git.Line{
		{Type: git.LineContext, Content: "line1", OldNum: 1, NewNum: 1},
		{Type: git.LineContext, Content: "line2", OldNum: 2, NewNum: 2},
		{Type: git.LineAdded, Content: "line3", NewNum: 3},
		{Type: git.LineContext, Content: "line4", OldNum: 3, NewNum: 4},
	}
	m := makeModelWithDiff("foo.go", lines)
	// Move cursor down a few times
	m = sendKey(m, "l") // focus diff
	m = sendKey(m, "j")
	m = sendKey(m, "j")
	savedCursor := m.diffView.cursor
	savedOffset := m.diffView.offset

	// Simulate a poll-triggered refresh
	updated, _ := m.Update(diffLoadedMsg{
		diff:     m.diff,
		fromPoll: true,
	})
	m = updated.(Model)
	assert.Equal(t, savedCursor, m.diffView.cursor, "poll refresh should preserve cursor")
	assert.Equal(t, savedOffset, m.diffView.offset, "poll refresh should preserve offset")
}

func TestRefreshResult_Error_NoReload(t *testing.T) {
	m := makeModel("a.go")
	m.lastFingerprint = "M a.go\n"

	_, cmd := m.Update(refreshResultMsg{err: fmt.Errorf("git error"), duration: 50 * time.Millisecond})
	assert.NotNil(t, cmd, "error should still schedule next tick")
}

func TestRefreshTick_SetsPollingFlag(t *testing.T) {
	m := makeModel("a.go")
	assert.False(t, m.polling)

	updated, cmd := m.Update(refreshTickMsg{gen: m.pollGen})
	m = updated.(Model)
	assert.True(t, m.polling, "polling flag should be set")
	assert.NotNil(t, cmd, "should return fingerprint check command")
}

func TestRefreshTick_SkipsWhenAlreadyPolling(t *testing.T) {
	m := makeModel("a.go")
	m.polling = true

	updated, cmd := m.Update(refreshTickMsg{gen: m.pollGen})
	m = updated.(Model)
	assert.True(t, m.polling, "polling flag should remain true")
	assert.Nil(t, cmd, "should not schedule anything when already polling")
}

func TestRefreshTick_SkipsWhenUnfocused(t *testing.T) {
	m := makeModel("a.go")
	m.focused = false

	updated, cmd := m.Update(refreshTickMsg{gen: m.pollGen})
	m = updated.(Model)
	assert.False(t, m.polling, "should not start polling when unfocused")
	assert.Nil(t, cmd, "should not schedule anything when unfocused")
}

func TestRefreshTick_StaleGenIsDropped(t *testing.T) {
	m := makeModel("a.go")
	m.pollGen = 5

	updated, cmd := m.Update(refreshTickMsg{gen: 3})
	m = updated.(Model)
	assert.False(t, m.polling, "stale tick should not start a poll")
	assert.Nil(t, cmd, "stale tick should not produce a command")
}

func TestBlur_StopsPolling(t *testing.T) {
	m := makeModel("a.go")
	assert.True(t, m.focused)

	updated, cmd := m.Update(tea.BlurMsg{})
	m = updated.(Model)
	assert.False(t, m.focused)
	assert.Nil(t, cmd, "blur should not schedule anything")
}

func TestFocus_ResumesPolling(t *testing.T) {
	m := makeModel("a.go")
	m.focused = false

	updated, cmd := m.Update(tea.FocusMsg{})
	m = updated.(Model)
	assert.True(t, m.focused)
	assert.NotNil(t, cmd, "focus should schedule a poll tick")
}

func TestFocus_PollsImmediately(t *testing.T) {
	m := makeModel("a.go")
	m.focused = false

	_, cmd := m.Update(tea.FocusMsg{})
	require.NotNil(t, cmd, "focus should return a command")

	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		_, ok := msg.(refreshTickMsg)
		assert.True(t, ok, "focus should fire refreshTickMsg immediately, got %T", msg)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("focus did not fire refreshTickMsg within 500ms — likely scheduled via tea.Tick instead")
	}
}

func TestFSEvent_RequestsRefreshAndRelistens(t *testing.T) {
	m := makeModel("a.go")

	// No watcher attached, so listenFSEvents returns nil; the relisten
	// path is exercised in main.go integration. This test focuses on the
	// requestRefresh side: an fsEvent should produce a Cmd that fires a
	// refreshTickMsg promptly (subject to debounce — and lastPollStart is
	// zero so debounce permits immediate firing).
	_, cmd := m.Update(fsEventMsg{})
	require.NotNil(t, cmd, "fsEventMsg should request a refresh")

	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		_, ok := msg.(refreshTickMsg)
		assert.True(t, ok, "fsEvent should fire refreshTickMsg immediately, got %T", msg)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("fsEvent did not produce refreshTickMsg promptly")
	}
}

func TestRequestRefresh_SkipsWhilePolling(t *testing.T) {
	m := makeModel("a.go")
	m.polling = true

	_, cmd := m.Update(tea.FocusMsg{})
	assert.Nil(t, cmd, "in-flight poll should suppress new requests; result will reschedule")
}

func TestRequestRefresh_BumpsGenToInvalidateOldTicks(t *testing.T) {
	m := makeModel("a.go")
	startGen := m.pollGen

	updated, _ := m.Update(tea.FocusMsg{})
	m = updated.(Model)
	gen1 := m.pollGen
	assert.Greater(t, gen1, startGen, "FocusMsg should bump pollGen")

	updated, _ = m.Update(fsEventMsg{})
	m = updated.(Model)
	assert.Greater(t, m.pollGen, gen1, "fsEventMsg should bump pollGen again")
}

func TestRefreshResult_SkipsNextTickWhenUnfocused(t *testing.T) {
	m := makeModel("a.go")
	m.polling = true
	m.focused = false
	m.lastFingerprint = "M a.go\n"

	updated, cmd := m.Update(refreshResultMsg{fingerprint: "M a.go\n", duration: 50 * time.Millisecond})
	m = updated.(Model)
	assert.False(t, m.polling, "polling flag should be cleared")
	assert.Nil(t, cmd, "should not schedule next tick when unfocused")
}

func TestRefreshResult_ClearsPollingFlag(t *testing.T) {
	m := makeModel("a.go")
	m.polling = true
	m.lastFingerprint = "M a.go\n"

	updated, _ := m.Update(refreshResultMsg{fingerprint: "M a.go\n", duration: 50 * time.Millisecond})
	m = updated.(Model)
	assert.False(t, m.polling, "polling flag should be cleared after result")
}

func TestShortDuration(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "0µs"},
		{500 * time.Microsecond, "500µs"},
		{45 * time.Millisecond, "45ms"},
		{2300 * time.Millisecond, "2.3s"},
		{125 * time.Second, "2m05s"},
		{-time.Second, "0µs"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, shortDuration(tc.in), "shortDuration(%s)", tc.in)
	}
}

func TestAgoAndDur_ZeroStartReturnsDash(t *testing.T) {
	assert.Equal(t, "—", agoAndDur(time.Time{}, 0, time.Now()))
}

func TestUntilOrDash(t *testing.T) {
	now := time.Now()
	assert.Equal(t, "—", untilOrDash(time.Time{}, now))
	assert.Equal(t, "now", untilOrDash(now.Add(-1*time.Second), now))
	assert.Equal(t, "1.5s", untilOrDash(now.Add(1500*time.Millisecond), now))
}

func TestDebugTickMsg_OnlyReschedulesInDebugMode(t *testing.T) {
	m := makeModel("a.go")
	m.debug = false

	_, cmd := m.Update(debugTickMsg{})
	assert.Nil(t, cmd, "debugTickMsg outside --debug should be a no-op")

	m.debug = true
	_, cmd = m.Update(debugTickMsg{})
	assert.NotNil(t, cmd, "debugTickMsg in --debug should reschedule")
}

func TestRefreshResult_RecordsDurationForBackoff(t *testing.T) {
	m := makeModel("a.go")
	m.polling = true

	updated, _ := m.Update(refreshResultMsg{fingerprint: "x", duration: 7 * time.Second})
	m = updated.(Model)
	assert.Equal(t, 7*time.Second, m.lastPollDuration,
		"lastPollDuration drives the next NextDelay() — slow polls must record their duration")
}

// --- File review mode tests ---

// makeFileReviewModel creates a file review mode model with context lines.
func makeFileReviewModel(lines []git.Line) Model {
	diff := &git.Diff{
		Files: []git.FileDiff{{
			Path:   "test.md",
			Status: git.StatusModified,
			Hunks:  []git.Hunk{{Lines: lines}},
		}},
	}
	m := NewFileReview(diff, "test.md")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(Model)
}

func TestFileReviewMode_StartsFullscreenWithDiffFocus(t *testing.T) {
	m := makeFileReviewModel([]git.Line{
		{Type: git.LineContext, Content: "hello", OldNum: 1, NewNum: 1},
	})
	assert.True(t, m.fullscreen)
	assert.Equal(t, focusDiffView, m.focus)
	assert.True(t, m.fileReviewMode)
}

func TestFileReviewMode_TabIsNoop(t *testing.T) {
	m := makeFileReviewModel([]git.Line{
		{Type: git.LineContext, Content: "hello", OldNum: 1, NewNum: 1},
	})
	mode := m.mode
	m = sendSpecialKey(m, tea.KeyTab)
	assert.Equal(t, mode, m.mode)
}

func TestFileReviewMode_PlusMinusAreNoops(t *testing.T) {
	m := makeFileReviewModel([]git.Line{
		{Type: git.LineContext, Content: "hello", OldNum: 1, NewNum: 1},
	})
	ctx := m.contextLines
	m = sendKey(m, "+")
	assert.Equal(t, ctx, m.contextLines)
	m = sendKey(m, "-")
	assert.Equal(t, ctx, m.contextLines)
}

func TestFileReviewMode_StageKeyIsNoop(t *testing.T) {
	m := makeFileReviewModel([]git.Line{
		{Type: git.LineContext, Content: "hello", OldNum: 1, NewNum: 1},
	})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	assert.Nil(t, cmd)
}

func TestFileReviewMode_UnstageKeyIsNoop(t *testing.T) {
	m := makeFileReviewModel([]git.Line{
		{Type: git.LineContext, Content: "hello", OldNum: 1, NewNum: 1},
	})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	assert.Nil(t, cmd)
}

func TestFileReviewMode_DiscardKeyIsNoop(t *testing.T) {
	m := makeFileReviewModel([]git.Line{
		{Type: git.LineContext, Content: "hello", OldNum: 1, NewNum: 1},
	})
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	m = updated.(Model)
	assert.Nil(t, cmd)
	assert.False(t, m.confirmDiscard)
}

func TestFileReviewMode_WhitespaceToggleIsNoop(t *testing.T) {
	m := makeFileReviewModel([]git.Line{
		{Type: git.LineContext, Content: "hello", OldNum: 1, NewNum: 1},
	})
	ws := m.hideWhitespace
	m = sendKey(m, "w")
	assert.Equal(t, ws, m.hideWhitespace)
}

func TestFileReviewMode_CommentStillWorks(t *testing.T) {
	m := makeFileReviewModel([]git.Line{
		{Type: git.LineContext, Content: "hello", OldNum: 1, NewNum: 1},
	})
	require.NotNil(t, m.diffView.cursorRef())
	m = sendKey(m, "c")
	assert.True(t, m.commentInputActive)
}

func TestFileReviewMode_ExportedComments(t *testing.T) {
	m := makeFileReviewModel([]git.Line{
		{Type: git.LineContext, Content: "hello world", OldNum: 1, NewNum: 1},
	})
	m.comments[commentKey{file: "test.md", lineNum: 1}] = "nice line"
	text := m.ExportedComments()
	assert.Contains(t, text, "# Code Review Comments")
	assert.Contains(t, text, "test.md")
	assert.Contains(t, text, "nice line")
}

func TestFileReviewMode_ExportedCommentsEmpty(t *testing.T) {
	m := makeFileReviewModel([]git.Line{
		{Type: git.LineContext, Content: "hello", OldNum: 1, NewNum: 1},
	})
	assert.Empty(t, m.ExportedComments())
}

func TestFileReviewMode_HelpOmitsGitBindings(t *testing.T) {
	groups := FileReviewBindingGroups()
	for _, g := range groups {
		for _, b := range g.Bindings {
			assert.False(t, b.GitOnly, "help should not include git-only binding: %s", b.Key)
		}
		// File List group should be entirely filtered out
		assert.NotEqual(t, "File List", g.Name)
	}
	// Verify specific git-only bindings are filtered
	allKeys := ""
	for _, g := range groups {
		for _, b := range g.Bindings {
			allKeys += b.Key + " "
		}
	}
	assert.NotContains(t, allKeys, "Tab/S-Tab")
	assert.NotContains(t, allKeys, "+/-")
	assert.NotContains(t, allKeys, "← (h)")
}

func TestFileReviewMode_CannotExitFullscreen(t *testing.T) {
	m := makeFileReviewModel([]git.Line{
		{Type: git.LineContext, Content: "hello", OldNum: 1, NewNum: 1},
	})
	require.True(t, m.fullscreen)
	// Left arrow, f, esc should all keep fullscreen
	m = sendKey(m, "h")
	assert.True(t, m.fullscreen)
	m = sendKey(m, "f")
	assert.True(t, m.fullscreen)
	m = sendSpecialKey(m, tea.KeyEsc)
	assert.True(t, m.fullscreen)
}

func TestFileReviewMode_InitReturnsNil(t *testing.T) {
	m := makeFileReviewModel([]git.Line{
		{Type: git.LineContext, Content: "hello", OldNum: 1, NewNum: 1},
	})
	assert.Nil(t, m.Init())
}

// TestPaneHeightsMatch is a regression test for #169: the file list and diff
// view must render to the same total number of rows regardless of how much
// content each pane has. Lipgloss v2 changed Height() to include borders, so
// short content silently rendered a shorter block and the help text appeared
// "above" the bottom of the file list. This is the invariant we now hold.
func TestPaneHeightsMatch(t *testing.T) {
	makeHunk := func(n int) git.Hunk {
		lines := make([]git.Line, 0, n)
		for i := 0; i < n; i++ {
			lines = append(lines, git.Line{Type: git.LineAdded, Content: "line"})
		}
		return git.Hunk{Header: "@@ -1 +1 @@", Lines: lines}
	}

	cases := []struct {
		name       string
		fileCount  int
		diffLines  int
		termHeight int
		termWidth  int
	}{
		{"few files, short diff", 2, 1, 40, 120},
		{"few files, long diff", 2, 200, 40, 120},
		{"many files, short diff", 100, 1, 40, 120},
		{"many files, long diff", 100, 200, 40, 120},
		{"narrow terminal", 2, 1, 40, 60},
		{"tall terminal", 5, 5, 80, 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := make([]git.FileDiff, tc.fileCount)
			hunk := makeHunk(tc.diffLines)
			for i := range files {
				files[i] = git.FileDiff{
					Path:   fmt.Sprintf("file_%d.txt", i),
					Status: git.StatusModified,
					Hunks:  []git.Hunk{hunk},
				}
			}
			m := New(&git.Diff{Files: files}, false)
			updated, _ := m.Update(tea.WindowSizeMsg{Width: tc.termWidth, Height: tc.termHeight})
			mm := updated.(Model)

			left := mm.fileList.render(true, "Branch+Staged+Unstaged")
			right := mm.diffView.render(false, mm.contextLines, mm.hideWhitespace)
			leftRows := strings.Count(left, "\n") + 1
			rightRows := strings.Count(right, "\n") + 1

			assert.Equal(t, leftRows, rightRows,
				"file list (%d rows) and diff view (%d rows) must have the same height",
				leftRows, rightRows)
		})
	}
}

func TestModelE_ReturnsExecCmdWhenFileSelected(t *testing.T) {
	t.Setenv("EDITOR", "true") // any benign binary
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineContext, Content: "line one", OldNum: 1, NewNum: 1},
	})
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E")})
	m = updated.(Model)
	assert.NotNil(t, cmd, "E should return a tea.ExecProcess command")
	assert.Empty(t, m.statusMsg)
}

func TestModelE_NoOpWithNoFile(t *testing.T) {
	t.Setenv("EDITOR", "true")
	m := New(&git.Diff{Files: nil}, false)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E")})
	m = updated.(Model)
	assert.Nil(t, cmd, "E with no file should be a no-op")
	assert.Empty(t, m.statusMsg)
}

func TestModelEditorFinished_ReloadsDiffInGitMode(t *testing.T) {
	m := makeModel("a.go")
	updated, cmd := m.Update(editorFinishedMsg{err: nil})
	m = updated.(Model)
	assert.NotNil(t, cmd, "editorFinishedMsg should trigger a diff reload")
	assert.Empty(t, m.statusMsg)
}

func TestModelEditorFinished_NoReloadInFileReviewMode(t *testing.T) {
	m := New(&git.Diff{Files: []git.FileDiff{{Path: "a.go", Status: git.StatusModified}}}, false)
	m.fileReviewMode = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	updated, cmd := m.Update(editorFinishedMsg{err: nil})
	m = updated.(Model)
	assert.Nil(t, cmd)
	assert.Empty(t, m.statusMsg)
}

func TestModelEditorFinished_ShowsStatusOnExecError(t *testing.T) {
	m := makeModel("a.go")
	// A bare error (not *exec.ExitError) represents "couldn't launch the
	// editor at all" — the user should see this.
	updated, cmd := m.Update(editorFinishedMsg{err: fmt.Errorf("boom")})
	m = updated.(Model)
	assert.Contains(t, m.statusMsg, "Editor error")
	assert.Contains(t, m.statusMsg, "boom")
	assert.NotNil(t, cmd, "should return a clearStatusMsg tick")
}

func TestModelEditorFinished_SilentOnExitError(t *testing.T) {
	m := makeModel("a.go")
	// Simulate vim's `:cq` (exit non-zero) by running a real command that exits 1.
	c := exec.Command("sh", "-c", "exit 1")
	_ = c.Run()
	exitErr, ok := c.ProcessState.ExitCode(), c.ProcessState.Exited()
	require.True(t, ok && exitErr != 0, "test setup: expected non-zero exit")

	updated, cmd := m.Update(editorFinishedMsg{err: &exec.ExitError{ProcessState: c.ProcessState}})
	m = updated.(Model)
	assert.Empty(t, m.statusMsg, "exit-code errors should not surface a status message")
	assert.NotNil(t, cmd, "should still reload the diff after the editor exits")
}

func TestModelE_DeletedFileShowsStatus(t *testing.T) {
	t.Setenv("EDITOR", "true")
	files := []git.FileDiff{{Path: "gone.go", Status: git.StatusDeleted}}
	m := New(&git.Diff{Files: files}, false)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E")})
	m = updated.(Model)
	assert.Contains(t, m.statusMsg, "deleted")
	assert.NotNil(t, cmd, "should return a clearStatusMsg tick")
}

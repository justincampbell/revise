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
	m := New(&git.Diff{Files: files})
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
	m := New(&git.Diff{Files: files})
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

func TestModelHelpDismissedByAnyKey(t *testing.T) {
	m := makeModel("a.go")
	m = sendKey(m, "?")
	require.True(t, m.showHelp)
	m = sendKey(m, "x")
	assert.False(t, m.showHelp)
}

// --- Comment tests ---

func TestModelComment_PressC_OnNonCodeLine_DoesNothing(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m = sendKey(m, "l") // focus diff view, cursor at line 0 (file header, non-code)
	m = sendKey(m, "c")
	assert.False(t, m.commentInputActive)
}

func TestModelComment_PressC_OnCodeLine_EntersCommentMode(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	// lines: [0: header, 1: blank, 2: hunk, 3: code, 4: blank]
	m = focusDiffAndMoveTo(m, 3)
	require.NotNil(t, m.diffView.cursorRef())
	m = sendKey(m, "c")
	assert.True(t, m.commentInputActive)
}

func TestModelComment_InputEsc_Cancels(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m = focusDiffAndMoveTo(m, 3)
	m = sendKey(m, "c")
	require.True(t, m.commentInputActive)
	m = sendSpecialKey(m, tea.KeyEsc)
	assert.False(t, m.commentInputActive)
	assert.Empty(t, m.commentInput)
}

func TestModelComment_TypeAndSave(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m = focusDiffAndMoveTo(m, 3)
	m = sendKey(m, "c")
	m = sendKey(m, "h")
	m = sendKey(m, "i")
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
	m = focusDiffAndMoveTo(m, 3)
	m = sendKey(m, "d")
	_, exists := m.comments[commentKey{file: "foo.go", lineNum: 1}]
	assert.False(t, exists)
}

func TestModelComment_EditExistingComment_PreFills(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m.comments[commentKey{file: "foo.go", lineNum: 1}] = "existing"
	m = focusDiffAndMoveTo(m, 3)
	m = sendKey(m, "c")
	assert.True(t, m.commentInputActive)
	assert.Equal(t, "existing", m.commentInput)
}

func TestModelComment_Backspace(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m = focusDiffAndMoveTo(m, 3)
	m = sendKey(m, "c")
	m = sendKey(m, "a")
	m = sendKey(m, "b")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(Model)
	assert.Equal(t, "a", m.commentInput)
}

func TestModelComment_ClearInputOnEnter_DeletesComment(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m.comments[commentKey{file: "foo.go", lineNum: 1}] = "hi"
	m = focusDiffAndMoveTo(m, 3)
	m = sendKey(m, "c")
	require.Equal(t, "hi", m.commentInput) // pre-filled
	// Clear the input with backspace, then press enter
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(Model)
	require.Empty(t, m.commentInput)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	_, exists := m.comments[commentKey{file: "foo.go", lineNum: 1}]
	assert.False(t, exists)
}

func TestModelComment_CommentInputBlocksOtherKeys(t *testing.T) {
	m := makeModelWithDiff("foo.go", []git.Line{
		{Type: git.LineAdded, Content: "hello", NewNum: 1},
	})
	m = focusDiffAndMoveTo(m, 3)
	m = sendKey(m, "c")
	require.True(t, m.commentInputActive)
	// Pressing "q" should add to input, not quit
	m = sendKey(m, "q")
	assert.True(t, m.commentInputActive)
	assert.Equal(t, "q", m.commentInput)
}

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

func sendKey(m Model, key string) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return updated.(Model)
}

func sendSpecialKey(m Model, keyType tea.KeyType) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: keyType})
	return updated.(Model)
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

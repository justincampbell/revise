package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/justincampbell/revise/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func pressKey(m Model, key string) (Model, tea.Cmd) {
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return updated.(Model), cmd
}

func pressSpecial(m Model, keyType tea.KeyType) (Model, tea.Cmd) {
	updated, cmd := m.Update(tea.KeyMsg{Type: keyType})
	return updated.(Model), cmd
}

func typeText(m Model, text string) Model {
	for _, r := range text {
		m, _ = pressKey(m, string(r))
	}
	return m
}

func makeVerdictModel() Model {
	m := makeModel("a.go").WithVerdict()
	m.comments[commentKey{file: "a.go", lineNum: 3}] = "fix this"
	return m
}

func TestQuit_WithoutVerdictQuitsImmediately(t *testing.T) {
	m := makeModel("a.go")
	_, cmd := pressKey(m, "q")
	assert.True(t, isQuit(cmd))
}

func TestQuit_WithVerdictOpensPrompt(t *testing.T) {
	m := makeVerdictModel()
	m, cmd := pressKey(m, "q")
	assert.False(t, isQuit(cmd))
	assert.True(t, m.verdictPrompt)
}

func TestVerdictPrompt_EscReturnsToReview(t *testing.T) {
	m := makeVerdictModel()
	m, _ = pressKey(m, "q")
	m, cmd := pressSpecial(m, tea.KeyEsc)
	assert.False(t, isQuit(cmd))
	assert.False(t, m.verdictPrompt)
	assert.Equal(t, VerdictNone, m.verdict)
}

func TestVerdictPrompt_QQuitsWithNoVerdict(t *testing.T) {
	m := makeVerdictModel()
	m, _ = pressKey(m, "q")
	m, cmd := pressKey(m, "q")
	assert.True(t, isQuit(cmd))
	assert.Equal(t, VerdictNone, m.verdict)
}

func TestVerdictPrompt_IgnoresOtherKeys(t *testing.T) {
	m := makeVerdictModel()
	m, _ = pressKey(m, "q")
	m, cmd := pressKey(m, "x")
	assert.False(t, isQuit(cmd))
	assert.True(t, m.verdictPrompt)
}

func TestVerdictPrompt_ApproveThenEnterQuits(t *testing.T) {
	m := makeVerdictModel()
	m, _ = pressKey(m, "q")
	m, cmd := pressKey(m, "a")
	require.False(t, isQuit(cmd))
	assert.True(t, m.reviewInputActive)

	m = typeText(m, "looks good")
	m, cmd = pressSpecial(m, tea.KeyEnter)
	assert.True(t, isQuit(cmd))
	assert.Equal(t, VerdictApprove, m.verdict)
	assert.Equal(t, "looks good", m.reviewComment)
}

func TestVerdictPrompt_RequestChangesWithEmptyComment(t *testing.T) {
	m := makeVerdictModel()
	m, _ = pressKey(m, "q")
	m, _ = pressKey(m, "r")
	m, cmd := pressSpecial(m, tea.KeyEnter)
	assert.True(t, isQuit(cmd))
	assert.Equal(t, VerdictRequestChanges, m.verdict)
	assert.Equal(t, "", m.reviewComment)
}

func TestReviewInput_EscReturnsToPrompt(t *testing.T) {
	m := makeVerdictModel()
	m, _ = pressKey(m, "q")
	m, _ = pressKey(m, "a")
	m, cmd := pressSpecial(m, tea.KeyEsc)
	assert.False(t, isQuit(cmd))
	assert.False(t, m.reviewInputActive)
	assert.True(t, m.verdictPrompt)
	assert.Equal(t, VerdictNone, m.verdict)
}

func TestReviewInput_KeysFeedInputNotCommands(t *testing.T) {
	m := makeVerdictModel()
	m, _ = pressKey(m, "q")
	m, _ = pressKey(m, "a")
	m, cmd := pressKey(m, "q")
	assert.False(t, isQuit(cmd))
	assert.Equal(t, "q", m.reviewInput.Value())
}

func TestCtrlC_QuitsWithNoVerdictFromPromptAndInput(t *testing.T) {
	m := makeVerdictModel()
	_, cmd := pressSpecial(m, tea.KeyCtrlC)
	assert.True(t, isQuit(cmd))

	m, _ = pressKey(m, "q")
	_, cmd = pressSpecial(m, tea.KeyCtrlC)
	assert.True(t, isQuit(cmd))

	m, _ = pressKey(m, "a")
	m, cmd = pressSpecial(m, tea.KeyCtrlC)
	assert.True(t, isQuit(cmd))
	assert.Equal(t, VerdictNone, m.verdict)
}

func makeVerdictModelWithStore(t *testing.T) (Model, string) {
	m, storePath := makeModelWithStore(t, "a.go", []git.Line{{Type: git.LineAdded, Content: "x", NewNum: 1}})
	m = m.WithVerdict()
	m.comments[commentKey{file: "a.go", lineNum: 1}] = "fix"
	m.saveComments()
	require.FileExists(t, storePath)
	return m, storePath
}

func TestVerdict_FinishingDeletesStore(t *testing.T) {
	for _, key := range []string{"a", "r"} {
		m, storePath := makeVerdictModelWithStore(t)
		m, _ = pressKey(m, "q")
		m, _ = pressKey(m, key)
		m, cmd := pressSpecial(m, tea.KeyEnter)
		require.True(t, isQuit(cmd))
		assert.NoFileExists(t, storePath, key)
		assert.Contains(t, m.ExportedComments(), "> fix", "export still has the comments")
	}
}

func TestVerdict_NoneKeepsStore(t *testing.T) {
	m, storePath := makeVerdictModelWithStore(t)
	m, _ = pressKey(m, "q")
	_, cmd := pressKey(m, "q")
	require.True(t, isQuit(cmd))
	assert.FileExists(t, storePath)

	m, storePath = makeVerdictModelWithStore(t)
	m, _ = pressKey(m, "q")
	m, _ = pressKey(m, "a")
	_, cmd = pressSpecial(m, tea.KeyCtrlC)
	require.True(t, isQuit(cmd))
	assert.FileExists(t, storePath)
}

func TestExportedComments_VerdictFirstLine(t *testing.T) {
	m := makeVerdictModel()
	m.verdict = VerdictApprove
	m.reviewComment = "ship it"
	out := m.ExportedComments()
	assert.True(t, strings.HasPrefix(out, "verdict: approve\n"), out)
	assert.Contains(t, out, "\n# Review\n\n> ship it\n")
	assert.Contains(t, out, "# Code Review Comments")
	assert.Less(t, strings.Index(out, "# Review\n"), strings.Index(out, "# Code Review Comments"))
}

func TestExportedComments_VerdictWithNothingElse(t *testing.T) {
	m := makeModel("a.go").WithVerdict()
	assert.Equal(t, "verdict: none\n", m.ExportedComments())
}

func TestExportedComments_NoVerdictUnchanged(t *testing.T) {
	m := makeModel("a.go")
	assert.Equal(t, "", m.ExportedComments())
	m.comments[commentKey{file: "a.go", lineNum: 3}] = "fix this"
	assert.True(t, strings.HasPrefix(m.ExportedComments(), "# Code Review Comments"))
}

func TestFormatReviewExport(t *testing.T) {
	assert.Equal(t, "verdict: request-changes\n\n# Review\n\n> needs tests\n",
		formatReviewExport(VerdictRequestChanges, "needs tests", ""))
	assert.Equal(t, "verdict: approve\n\n# Code Review Comments\n\n## a.go",
		formatReviewExport(VerdictApprove, "", "# Code Review Comments\n\n## a.go"))
}

func makeBriefModel(brief string, height int) Model {
	m := makeModel("a.go").WithBrief(brief)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: height})
	return updated.(Model)
}

func TestBriefHeight_NoBrief(t *testing.T) {
	m := makeModel("a.go")
	assert.Equal(t, 0, m.briefHeight())
}

func TestBriefHeight_ExpandedFitsContent(t *testing.T) {
	m := makeBriefModel("# PR 12\n\nAdds a thing.\n\n", 40)
	assert.True(t, m.briefExpanded)
	assert.Equal(t, 3+2, m.briefHeight(), "trailing blank lines trimmed; +2 for border")
}

func TestBriefHeight_ExpandedIsCapped(t *testing.T) {
	m := makeBriefModel(strings.Repeat("line\n", 100), 40)
	assert.Equal(t, briefMaxLines(40)+2, m.briefHeight())
}

func TestBriefHeight_WrapsLongLines(t *testing.T) {
	m := makeBriefModel(strings.Repeat("word ", 50), 40)
	assert.Greater(t, m.briefHeight(), 3)
}

func TestBriefToggle_CollapsesToOneRowAndShrinksPanels(t *testing.T) {
	m := makeBriefModel("# PR 12\n\nAdds a thing.", 40)
	expandedPanel := m.diffView.height
	assert.Equal(t, 40-3-m.briefHeight(), expandedPanel)

	m, _ = pressKey(m, "b")
	assert.False(t, m.briefExpanded)
	assert.Equal(t, 1, m.briefHeight())
	assert.Equal(t, 40-3-1, m.diffView.height)
	assert.Equal(t, 40-3-1, m.fileList.height)

	m, _ = pressKey(m, "b")
	assert.True(t, m.briefExpanded)
	assert.Equal(t, expandedPanel, m.diffView.height)
}

func TestBriefToggle_NoBriefIsNoop(t *testing.T) {
	m := makeModel("a.go")
	before := m.diffView.height
	m, _ = pressKey(m, "b")
	assert.Equal(t, before, m.diffView.height)
	assert.Equal(t, 0, m.briefHeight())
}

func TestBriefTitle(t *testing.T) {
	assert.Equal(t, "PR 12: add thing", briefTitle([]string{"", "## PR 12: add thing", "body"}))
	assert.Equal(t, "", briefTitle([]string{"", " "}))
}

func TestBriefView_RendersAboveFileList(t *testing.T) {
	m := makeBriefModel("# PR 12\n\nAdds a thing.", 40)
	view := m.View()
	assert.Less(t, strings.Index(view, "Adds a thing."), strings.Index(view, "a.go"))
	assert.Equal(t, 40, len(strings.Split(view, "\n")))

	m, _ = pressKey(m, "b")
	view = m.View()
	assert.NotContains(t, view, "Adds a thing.")
	assert.Contains(t, view, "PR 12")
	assert.Equal(t, 40, len(strings.Split(view, "\n")))
}

func TestBriefCollapsed_LongTitleStaysOneRow(t *testing.T) {
	m := makeBriefModel("# "+strings.Repeat("long title ", 30), 40)
	m, _ = pressKey(m, "b")
	strip := m.renderBrief()
	assert.NotContains(t, strip, "\n")
	assert.LessOrEqual(t, ansi.StringWidth(strip), m.width)
	assert.Equal(t, 40, len(strings.Split(m.View(), "\n")))
}

func TestBriefMouseClick_OffsetsFileListRows(t *testing.T) {
	m := makeModel("a.go", "b.go", "c.go").WithBrief("one\ntwo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	top := m.briefHeight()

	updated, _ = m.Update(tea.MouseMsg{X: 5, Y: top + 1 + 2, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = updated.(Model)
	assert.Equal(t, 2, m.fileList.cursor)
}

func TestBriefMouseClick_InBriefTogglesIt(t *testing.T) {
	m := makeBriefModel("one\ntwo", 40)
	updated, _ := m.Update(tea.MouseMsg{X: 5, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = updated.(Model)
	assert.False(t, m.briefExpanded)
	assert.Equal(t, 0, m.fileList.cursor)
}

func TestReviewInput_LongCommentWrapsInsideDialog(t *testing.T) {
	m := makeVerdictModel()
	m, _ = pressKey(m, "q")
	m, _ = pressKey(m, "r")
	m = typeText(m, longComment+" "+longComment)

	dialog := m.renderVerdictDialog()
	for _, r := range plainRows(dialog) {
		assert.LessOrEqual(t, ansi.StringWidth(r), m.width)
	}
	words := strings.Fields(ansi.Strip(dialog))
	assert.Contains(t, words, "box")
	assert.Contains(t, words, "the")
	assert.Equal(t, longComment+" "+longComment, m.reviewInput.Value())
}

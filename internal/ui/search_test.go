package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justincampbell/revise/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeSearchModel builds a file-review-style diff view whose source lines
// contain the substring "alpha" on lines 0, 2, and 4 (case-insensitively):
//
//	0: alpha beta
//	1: gamma delta
//	2: Alpha again
//	3: epsilon
//	4: beta gamma alpha
func makeSearchModel(t *testing.T) diffViewModel {
	t.Helper()
	m := newDiffViewModel()
	m.height = 20
	m.width = 80
	m.fileReviewMode = true
	m.file = &git.FileDiff{
		Path: "doc.md",
		Hunks: []git.Hunk{{
			Lines: []git.Line{
				{Type: git.LineContext, Content: "alpha beta", NewNum: 1},
				{Type: git.LineContext, Content: "gamma delta", NewNum: 2},
				{Type: git.LineContext, Content: "Alpha again", NewNum: 3},
				{Type: git.LineContext, Content: "epsilon", NewNum: 4},
				{Type: git.LineContext, Content: "beta gamma alpha", NewNum: 5},
			},
		}},
	}
	m.buildLines()
	m.goToFirstNavigable()
	require.Equal(t, 0, m.cursor)
	return m
}

func TestMatchRanges_CaseInsensitive(t *testing.T) {
	assert.Equal(t, [][2]int{{0, 5}}, matchRanges("Alpha again", "alpha"))
	assert.Equal(t, [][2]int{{11, 16}}, matchRanges("beta gamma alpha", "alpha"))
	assert.Nil(t, matchRanges("nothing here", "alpha"))
	assert.Nil(t, matchRanges("anything", ""))
}

func TestMatchRanges_MultiplePerLine(t *testing.T) {
	// "aXaXa" with query "a" → three matches.
	assert.Equal(t, [][2]int{{0, 1}, {2, 3}, {4, 5}}, matchRanges("aXaXa", "a"))
}

func TestMatchColumnRanges_ASCII(t *testing.T) {
	// For ASCII content, columns equal rune indices.
	assert.Equal(t, [][2]int{{0, 5}}, matchColumnRanges("Alpha again", "alpha"))
}

func TestMatchColumnRanges_WideRunes(t *testing.T) {
	// "日本 world": each CJK rune is 2 columns wide. "world" starts at rune
	// index 3 but column 5 (2+2+1).
	assert.Equal(t, [][2]int{{5, 10}}, matchColumnRanges("日本 world", "world"))
}

func makeLongLineSearchModel(t *testing.T, content string, width int) diffViewModel {
	t.Helper()
	m := newDiffViewModel()
	m.height = 10
	m.width = width
	m.fileReviewMode = true
	m.file = &git.FileDiff{
		Path:  "doc.md",
		Hunks: []git.Hunk{{Lines: []git.Line{{Type: git.LineContext, Content: content, NewNum: 1}}}},
	}
	m.buildLines()
	m.goToFirstNavigable()
	return m
}

func TestEnsureMatchVisible_ScrollsToOffscreenMatch(t *testing.T) {
	content := strings.Repeat("x", 40) + "needle"
	m := makeLongLineSearchModel(t, content, 23) // viewWidth = 20
	m.searchOrigin = 0
	m.setSearch("needle")

	avail := m.width - 3
	start := gutterWidth + 40 // match start column in the rendered line
	end := gutterWidth + 46   // match end column
	assert.Greater(t, m.hOffset, 0, "should scroll right to reveal the match")
	assert.GreaterOrEqual(t, start, m.hOffset, "match start should be at/after the left edge")
	assert.LessOrEqual(t, end, m.hOffset+avail, "match end should be within the viewport")
}

func TestEnsureMatchVisible_NoScrollWhenVisible(t *testing.T) {
	m := makeSearchModel(t)
	m.searchOrigin = 0
	m.setSearch("alpha") // match near the start, line fits in width 80
	assert.Equal(t, 0, m.hOffset)
}

func TestEnsureMatchVisible_NoScrollWhenWrapEnabled(t *testing.T) {
	content := strings.Repeat("x", 40) + "needle"
	m := makeLongLineSearchModel(t, content, 23)
	m.wrapEnabled = true
	m.searchOrigin = 0
	m.setSearch("needle")
	assert.Equal(t, 0, m.hOffset, "wrap on: no horizontal scroll")
}

func TestComputeSearchMatches(t *testing.T) {
	m := makeSearchModel(t)
	m.searchQuery = "alpha"
	m.computeSearchMatches()
	assert.Equal(t, []int{0, 2, 4}, m.searchMatches)
}

func TestComputeSearchMatches_EmptyQuery(t *testing.T) {
	m := makeSearchModel(t)
	m.searchQuery = ""
	m.computeSearchMatches()
	assert.Empty(t, m.searchMatches)
	assert.Equal(t, -1, m.searchIdx)
}

func TestSetSearch_JumpsToFirstMatchFromOrigin(t *testing.T) {
	m := makeSearchModel(t)
	m.searchOrigin = 0
	m.setSearch("alpha")
	assert.Equal(t, []int{0, 2, 4}, m.searchMatches)
	assert.Equal(t, 0, m.searchIdx)
	assert.Equal(t, 0, m.cursor)
}

func TestSetSearch_JumpsToNextMatchAtOrAfterOrigin(t *testing.T) {
	m := makeSearchModel(t)
	m.searchOrigin = 1 // first match at or after line 1 is line 2
	m.setSearch("alpha")
	assert.Equal(t, 1, m.searchIdx)
	assert.Equal(t, 2, m.cursor)
}

func TestSetSearch_WrapsWhenNoMatchAfterOrigin(t *testing.T) {
	m := makeSearchModel(t)
	m.searchOrigin = 4 // matches at 0,2,4; origin 4 lands on the last match
	m.setSearch("alpha")
	assert.Equal(t, 2, m.searchIdx)
	assert.Equal(t, 4, m.cursor)

	m2 := makeSearchModel(t)
	m2.cursor = 3
	m2.searchOrigin = 3 // no match at >=3 except 4 → lands on 4
	m2.setSearch("epsilon")
	require.Equal(t, []int{3}, m2.searchMatches)
	assert.Equal(t, 0, m2.searchIdx)
	assert.Equal(t, 3, m2.cursor)
}

func TestSetSearch_NoMatchKeepsCursor(t *testing.T) {
	m := makeSearchModel(t)
	m.cursor = 1
	m.searchOrigin = 1
	m.setSearch("zzzz")
	assert.Empty(t, m.searchMatches)
	assert.Equal(t, -1, m.searchIdx)
	assert.Equal(t, 1, m.cursor, "cursor should not move when there are no matches")
}

func TestNextMatch_WrapsForward(t *testing.T) {
	m := makeSearchModel(t)
	m.searchOrigin = 0
	m.setSearch("alpha") // idx 0, cursor 0
	m.nextMatch()
	assert.Equal(t, 1, m.searchIdx)
	assert.Equal(t, 2, m.cursor)
	m.nextMatch()
	assert.Equal(t, 2, m.searchIdx)
	assert.Equal(t, 4, m.cursor)
	m.nextMatch() // wrap
	assert.Equal(t, 0, m.searchIdx)
	assert.Equal(t, 0, m.cursor)
}

func TestPrevMatch_WrapsBackward(t *testing.T) {
	m := makeSearchModel(t)
	m.searchOrigin = 0
	m.setSearch("alpha") // idx 0, cursor 0
	m.prevMatch()        // wrap to last
	assert.Equal(t, 2, m.searchIdx)
	assert.Equal(t, 4, m.cursor)
	m.prevMatch()
	assert.Equal(t, 1, m.searchIdx)
	assert.Equal(t, 2, m.cursor)
}

func TestNextPrevMatch_NoMatchesNoop(t *testing.T) {
	m := makeSearchModel(t)
	m.cursor = 2
	m.setSearch("zzzz")
	m.nextMatch()
	assert.Equal(t, 2, m.cursor)
	m.prevMatch()
	assert.Equal(t, 2, m.cursor)
}

func TestClearSearch(t *testing.T) {
	m := makeSearchModel(t)
	m.searchOrigin = 0
	m.setSearch("alpha")
	require.NotEmpty(t, m.searchMatches)
	m.clearSearch()
	assert.Equal(t, "", m.searchQuery)
	assert.Empty(t, m.searchMatches)
	assert.Equal(t, -1, m.searchIdx)
}

func TestSetFile_ClearsSearch(t *testing.T) {
	m := makeSearchModel(t)
	m.searchOrigin = 0
	m.setSearch("alpha")
	require.NotEmpty(t, m.searchMatches)
	m.setFile(&git.FileDiff{
		Path:  "other.md",
		Hunks: []git.Hunk{{Lines: []git.Line{{Type: git.LineContext, Content: "x", NewNum: 1}}}},
	})
	assert.Equal(t, "", m.searchQuery)
	assert.Empty(t, m.searchMatches)
}

// --- Model-level routing ---

func makeSearchableFileReview() Model {
	return makeFileReviewModel([]git.Line{
		{Type: git.LineContext, Content: "alpha beta", OldNum: 1, NewNum: 1},
		{Type: git.LineContext, Content: "gamma delta", OldNum: 2, NewNum: 2},
		{Type: git.LineContext, Content: "Alpha again", OldNum: 3, NewNum: 3},
		{Type: git.LineContext, Content: "epsilon", OldNum: 4, NewNum: 4},
		{Type: git.LineContext, Content: "beta gamma alpha", OldNum: 5, NewNum: 5},
	})
}

func TestSearch_SlashOpensSearch(t *testing.T) {
	m := makeSearchableFileReview()
	m = sendKey(m, "/")
	assert.True(t, m.searchActive)
	assert.Equal(t, focusDiffView, m.focus)
}

func TestSearch_IncrementalTypingHighlightsAndJumps(t *testing.T) {
	m := makeSearchableFileReview()
	m = sendKey(m, "/")
	m = sendKey(m, "alpha")
	assert.Equal(t, "alpha", m.diffView.searchQuery)
	assert.Equal(t, []int{0, 2, 4}, m.diffView.searchMatches)
	assert.Equal(t, 0, m.diffView.cursor)
}

func TestSearch_EnterCommitsAndRetainsQuery(t *testing.T) {
	m := makeSearchableFileReview()
	m = sendKey(m, "/")
	m = sendKey(m, "alpha")
	m = sendSpecialKey(m, tea.KeyEnter)
	assert.False(t, m.searchActive)
	assert.Equal(t, "alpha", m.diffView.searchQuery)
	assert.Equal(t, []int{0, 2, 4}, m.diffView.searchMatches)
}

func TestSearch_EscInBoxClears(t *testing.T) {
	m := makeSearchableFileReview()
	m = sendKey(m, "/")
	m = sendKey(m, "alpha")
	m = sendSpecialKey(m, tea.KeyEsc)
	assert.False(t, m.searchActive)
	assert.Equal(t, "", m.diffView.searchQuery)
	assert.Empty(t, m.diffView.searchMatches)
}

func TestSearch_EscAfterCommitClears(t *testing.T) {
	m := makeSearchableFileReview()
	m = sendKey(m, "/")
	m = sendKey(m, "alpha")
	m = sendSpecialKey(m, tea.KeyEnter)
	require.Equal(t, "alpha", m.diffView.searchQuery)
	m = sendSpecialKey(m, tea.KeyEsc)
	assert.Equal(t, "", m.diffView.searchQuery)
}

func TestSearch_BackspaceOnEmptyExits(t *testing.T) {
	m := makeSearchableFileReview()
	m = sendKey(m, "/")
	require.True(t, m.searchActive)
	m = sendSpecialKey(m, tea.KeyBackspace)
	assert.False(t, m.searchActive, "backspace on empty search box exits search mode")
	assert.Equal(t, "", m.diffView.searchQuery)
}

func TestSearch_BackspaceDeletesWhenNonEmpty(t *testing.T) {
	m := makeSearchableFileReview()
	m = sendKey(m, "/")
	m = sendKey(m, "al")
	require.Equal(t, "al", m.diffView.searchQuery)
	m = sendSpecialKey(m, tea.KeyBackspace)
	assert.True(t, m.searchActive, "still searching after deleting one character")
	assert.Equal(t, "a", m.diffView.searchQuery)
}

func TestSearch_NNavigatesMatchesWhenActive(t *testing.T) {
	m := makeSearchableFileReview()
	m = sendKey(m, "/")
	m = sendKey(m, "alpha")
	m = sendSpecialKey(m, tea.KeyEnter)
	require.Equal(t, 0, m.diffView.cursor)
	m = sendKey(m, "n")
	assert.Equal(t, 2, m.diffView.cursor)
	m = sendKey(m, "n")
	assert.Equal(t, 4, m.diffView.cursor)
	m = sendKey(m, "N")
	assert.Equal(t, 2, m.diffView.cursor)
}

func TestSearch_NNavigatesFilesWhenNoSearch(t *testing.T) {
	m := makeModel("a.go", "b.go")
	require.Equal(t, 0, m.fileList.cursor)
	m = sendKey(m, "n")
	assert.Equal(t, 1, m.fileList.cursor, "n moves to the next file when not searching")
	m = sendKey(m, "N")
	assert.Equal(t, 0, m.fileList.cursor)
}

func TestSearch_HelpShowsSearchGroup(t *testing.T) {
	found := false
	for _, g := range BindingGroups() {
		if g.Name == "Search" {
			found = true
			keys := ""
			for _, b := range g.Bindings {
				keys += b.Key + " "
			}
			assert.Contains(t, keys, "/")
			assert.Contains(t, keys, "n/N")
		}
	}
	assert.True(t, found, "help should include a Search binding group")
}

func TestSearch_GroupVisibleInFileReviewHelp(t *testing.T) {
	// Search bindings are not git-only, so they survive the file-review filter.
	found := false
	for _, g := range FileReviewBindingGroups() {
		if g.Name == "Search" {
			found = true
		}
	}
	assert.True(t, found, "Search group should appear in file review help")
}

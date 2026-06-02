package ui

import (
	"testing"

	"github.com/justincampbell/revise/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeParagraphModel builds a file-review-style model whose source lines form
// three paragraphs separated by blank lines:
//
//	0: para1 line a
//	1: para1 line b
//	2: (blank)
//	3: para2 line a
//	4: (blank)
//	5: para3 line a
//	6: para3 line b
//	7: (trailing separator, non-navigable)
func makeParagraphModel(t *testing.T) diffViewModel {
	t.Helper()
	m := newDiffViewModel()
	m.height = 20
	m.fileReviewMode = true
	m.file = &git.FileDiff{
		Path: "doc.md",
		Hunks: []git.Hunk{{
			Lines: []git.Line{
				{Type: git.LineContext, Content: "para1 line a", NewNum: 1},
				{Type: git.LineContext, Content: "para1 line b", NewNum: 2},
				{Type: git.LineContext, Content: "", NewNum: 3},
				{Type: git.LineContext, Content: "para2 line a", NewNum: 4},
				{Type: git.LineContext, Content: "   ", NewNum: 5}, // whitespace-only counts as blank
				{Type: git.LineContext, Content: "para3 line a", NewNum: 6},
				{Type: git.LineContext, Content: "para3 line b", NewNum: 7},
			},
		}},
	}
	m.buildLines()
	m.goToFirstNavigable()
	// Sanity: blank lines detected at indices 2 and 4.
	require.True(t, m.isBlankLine(2))
	require.True(t, m.isBlankLine(4))
	require.False(t, m.isBlankLine(0))
	return m
}

func TestNextParagraph_FromTextLandsOnNextBlank(t *testing.T) {
	m := makeParagraphModel(t)
	m.cursor = 0
	m.nextParagraph()
	assert.Equal(t, 2, m.cursor)
}

func TestNextParagraph_FromBlankSkipsToFollowingBlank(t *testing.T) {
	m := makeParagraphModel(t)
	m.cursor = 2
	m.nextParagraph()
	assert.Equal(t, 4, m.cursor)
}

func TestNextParagraph_NoBlankBelowJumpsToLastLine(t *testing.T) {
	m := makeParagraphModel(t)
	m.cursor = 4 // last blank; only text remains below
	m.nextParagraph()
	assert.Equal(t, 6, m.cursor, "should jump to the last navigable line")
}

func TestPrevParagraph_FromTextLandsOnPrevBlank(t *testing.T) {
	m := makeParagraphModel(t)
	m.cursor = 5
	m.prevParagraph()
	assert.Equal(t, 4, m.cursor)
}

func TestPrevParagraph_FromBlankSkipsToPrecedingBlank(t *testing.T) {
	m := makeParagraphModel(t)
	m.cursor = 4
	m.prevParagraph()
	assert.Equal(t, 2, m.cursor)
}

func TestPrevParagraph_NoBlankAboveJumpsToTop(t *testing.T) {
	m := makeParagraphModel(t)
	m.cursor = 1 // inside the first paragraph, no blank above
	m.prevParagraph()
	assert.Equal(t, 0, m.cursor)
}

func TestParagraphJump_RoutedInAllModes(t *testing.T) {
	// }/{ jump to blank lines regardless of mode (no separate hunk nav).
	for _, fileReview := range []bool{true, false} {
		dv := makeParagraphModel(t)
		dv.fileReviewMode = fileReview
		m := Model{diffView: dv, fileReviewMode: fileReview, focus: focusDiffView}

		m = sendKey(m, "}")
		assert.Equal(t, 2, m.diffView.cursor, "} jumps to next blank line (fileReview=%v)", fileReview)

		m = sendKey(m, "{")
		assert.Equal(t, 0, m.diffView.cursor, "{ jumps to prev boundary (fileReview=%v)", fileReview)
	}
}

func TestHelp_ShowsBlankLineLabel(t *testing.T) {
	found := false
	for _, g := range BindingGroups() {
		for _, b := range g.Bindings {
			if b.Key == "}/{ (]/[)" {
				found = true
				assert.Equal(t, "Next/prev blank line", b.Desc)
			}
		}
	}
	assert.True(t, found, "}/{ binding should appear in help")
}

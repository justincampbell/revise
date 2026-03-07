package ui

import (
	"testing"

	"github.com/justincampbell/revise/internal/git"
	"github.com/stretchr/testify/assert"
)

func makeDiffViewModel(lineCount, height int) diffViewModel {
	m := newDiffViewModel()
	m.height = height
	m.lines = make([]string, lineCount)
	for i := range m.lines {
		m.lines[i] = "line"
	}
	return m
}

func TestDiffViewScrollDown_Clamps(t *testing.T) {
	m := makeDiffViewModel(10, 6) // viewHeight = 4, max offset = 6
	m.scrollDown(100)
	assert.Equal(t, 6, m.offset)
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
	m := makeDiffViewModel(10, 6) // viewHeight = 4, max = 6
	m.goToBottom()
	assert.Equal(t, 6, m.offset)
}

func TestDiffViewGoToBottom_ShortContent(t *testing.T) {
	m := makeDiffViewModel(2, 10) // content fits, max = 0
	m.goToBottom()
	assert.Equal(t, 0, m.offset)
}

func TestDiffViewViewHeight(t *testing.T) {
	m := newDiffViewModel()
	m.height = 20
	assert.Equal(t, 18, m.viewHeight())
}

func TestDiffViewViewHeight_MinimumOne(t *testing.T) {
	m := newDiffViewModel()
	m.height = 0
	assert.Equal(t, 1, m.viewHeight())
}

func TestFormatGutter_Added(t *testing.T) {
	l := git.Line{Type: git.LineAdded, NewNum: 42}
	assert.Equal(t, "       42 ", formatGutter(l))
}

func TestFormatGutter_Removed(t *testing.T) {
	l := git.Line{Type: git.LineRemoved, OldNum: 7}
	assert.Equal(t, "   7      ", formatGutter(l))
}

func TestFormatGutter_Context(t *testing.T) {
	l := git.Line{Type: git.LineContext, OldNum: 3, NewNum: 5}
	assert.Equal(t, "   3    5 ", formatGutter(l))
}

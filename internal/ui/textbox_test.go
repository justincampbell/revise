package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/justincampbell/revise/internal/git"
	"github.com/stretchr/testify/assert"
)

const longComment = "the quick brown fox jumps over the lazy dog and keeps running past the end of the box"

func plainRows(view string) []string {
	var rows []string
	for _, r := range strings.Split(ansi.Strip(view), "\n") {
		rows = append(rows, strings.TrimRight(r, " "))
	}
	return rows
}

func TestFitTextBox_GrowsToWrappedText(t *testing.T) {
	tb := newTextBox("", 500)
	tb.SetValue(longComment)
	fitTextBox(&tb, 20, 10)

	assert.Greater(t, tb.Height(), 1)
	rows := plainRows(tb.View())
	assert.Len(t, rows, tb.Height())
	assert.Equal(t, longComment, strings.Join(strings.Fields(strings.Join(rows, " ")), " "))
	for _, r := range rows {
		assert.LessOrEqual(t, ansi.StringWidth(r), 20)
	}
}

func TestFitTextBox_CapsAtMaxRows(t *testing.T) {
	tb := newTextBox("", 500)
	tb.SetValue(longComment)
	fitTextBox(&tb, 10, 3)
	assert.Equal(t, 3, tb.Height())
}

func TestFitTextBox_ShortTextIsOneRow(t *testing.T) {
	tb := newTextBox("", 500)
	tb.SetValue("hi")
	fitTextBox(&tb, 20, 10)
	assert.Equal(t, 1, tb.Height())
}

func TestFitTextBox_TypingPastWidthKeepsFirstRowVisible(t *testing.T) {
	tb := newTextBox("", 500)
	tb.Focus()
	fitTextBox(&tb, 20, 10)
	for _, r := range longComment {
		tb, _ = tb.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		fitTextBox(&tb, 20, 10)
	}
	rows := plainRows(tb.View())
	assert.True(t, strings.HasPrefix(rows[0], "the quick"), "first row: %q", rows[0])
	assert.Contains(t, rows[len(rows)-1], "box")
}

func TestTextBox_EnterDoesNotInsertNewline(t *testing.T) {
	tb := newTextBox("", 500)
	tb.Focus()
	tb.SetValue("hi")
	tb, _ = tb.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, "hi", tb.Value())
}

func TestFitTextBox_FlattensPastedNewlines(t *testing.T) {
	tb := newTextBox("", 500)
	tb.SetValue("one\ntwo")
	fitTextBox(&tb, 20, 10)
	assert.Equal(t, "one two", tb.Value())
	assert.Equal(t, 1, tb.LineCount())
}

func TestInputs_MultiRuneBurstIsInsertedNotTreatedAsKeyName(t *testing.T) {
	burst := func(m Model, words ...string) Model {
		for _, w := range words {
			m = sendKey(m, w)
		}
		return m
	}
	words := []string{"go ", "right", " ", "end", " ", "home", " ", "left"}
	want := "go right end home left"

	t.Run("comment", func(t *testing.T) {
		m := makeModelWithDiff("foo.go", []git.Line{{Type: git.LineAdded, Content: "x", NewNum: 1}})
		m = focusDiffAndMoveTo(m, 0)
		m = burst(sendKey(m, "c"), words...)
		assert.Equal(t, want, m.diffView.textInput.Value())
	})
	t.Run("review", func(t *testing.T) {
		m := makeVerdictModel()
		m, _ = pressKey(m, "q")
		m, _ = pressKey(m, "r")
		m = burst(m, words...)
		assert.Equal(t, want, m.reviewInput.Value())
	})
	t.Run("search", func(t *testing.T) {
		m := burst(sendKey(makeSearchableFileReview(), "/"), words...)
		assert.Equal(t, want, m.diffView.searchInput.Value())
	})
}

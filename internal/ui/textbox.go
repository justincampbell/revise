package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	lipglossv1 "github.com/charmbracelet/lipgloss" // bubbles v1 textarea styles are lipgloss v1
)

// asTyped marks a multi-rune burst (fast typing or an unbracketed paste) as a
// paste, so a word like "right" or "end" is inserted instead of matching the
// input's cursor-movement bindings, which compare against msg.String().
func asTyped(msg tea.KeyMsg) tea.KeyMsg {
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 1 {
		msg.Paste = true
	}
	return msg
}

// newTextBox returns a single-line free-text input that soft-wraps instead of
// scrolling sideways. Callers handle Enter themselves, so it never inserts a newline.
func newTextBox(placeholder string, charLimit int) textarea.Model {
	tb := textarea.New()
	tb.Placeholder = placeholder
	tb.CharLimit = charLimit
	tb.Prompt = ""
	tb.ShowLineNumbers = false
	tb.KeyMap.InsertNewline.SetEnabled(false)
	tb.FocusedStyle.CursorLine = lipglossv1.NewStyle()
	tb.FocusedStyle.Base = lipglossv1.NewStyle()
	tb.BlurredStyle.Base = lipglossv1.NewStyle()
	tb.SetHeight(1)
	return tb
}

// fitTextBox sets the box to width and grows or shrinks it to the wrapped text, up to maxRows.
func fitTextBox(tb *textarea.Model, width, maxRows int) {
	maxRows = max(1, maxRows)
	tb.SetWidth(width)
	if tb.LineCount() > 1 {
		tb.SetValue(strings.ReplaceAll(tb.Value(), "\n", " "))
	}
	rows := tb.LineInfo().Height
	tb.SetHeight(min(rows, maxRows))
	if rows <= maxRows {
		// The textarea never scrolls back up after growing, so reset to the top while preserving the cursor.
		li := tb.LineInfo()
		col := li.StartColumn + li.ColumnOffset
		tb.SetValue(tb.Value())
		tb.SetCursor(col)
	}
}

package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/justincampbell/revise/internal/git"
	"github.com/stretchr/testify/assert"
)

// extractBorders returns the first and last lines (top and bottom border)
// of a rendered panel, with ANSI codes stripped for deterministic comparison.
func extractBorders(rendered string) (top, bottom string) {
	stripped := ansi.Strip(rendered)
	lines := strings.Split(stripped, "\n")
	if len(lines) == 0 {
		return "", ""
	}
	top = lines[0]
	bottom = lines[len(lines)-1]
	return top, bottom
}

// TestDiffPaneBorderSnapshots asserts the exact top/bottom border composition
// for the diff pane across representative widths and states.
func TestDiffPaneBorderSnapshots(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		focused    bool
		file       *git.FileDiff
		context    int
		hideWS     bool
		wantTop    string
		wantBottom string
	}{
		{
			name:    "focused/simple file/width 40",
			width:   40,
			height:  8,
			focused: true,
			file: &git.FileDiff{
				Path: "foo.go",
				Hunks: []git.Hunk{{
					Header: "@@ -1,1 +1,2 @@",
					Lines: []git.Line{
						{Type: git.LineAdded, Content: "a", NewNum: 1},
						{Type: git.LineRemoved, Content: "b", OldNum: 1},
					},
				}},
			},
			context:    3,
			hideWS:     false,
			wantTop:    "╭ foo.go ────────────────────────────────╮",
			wantBottom: "╰ Context: 3 ───── +1/-1 ──── Whitespace ╯",
		},
		{
			name:    "unfocused/simple file/width 40",
			width:   40,
			height:  8,
			focused: false,
			file: &git.FileDiff{
				Path: "foo.go",
				Hunks: []git.Hunk{{
					Header: "@@ -1,1 +1,2 @@",
					Lines: []git.Line{
						{Type: git.LineAdded, Content: "a", NewNum: 1},
						{Type: git.LineRemoved, Content: "b", OldNum: 1},
					},
				}},
			},
			context:    3,
			hideWS:     false,
			wantTop:    "╭ foo.go ────────────────────────────────╮",
			wantBottom: "╰ Context: 3 ───── +1/-1 ──── Whitespace ╯",
		},
		{
			name:    "renamed file title/width 60",
			width:   60,
			height:  8,
			focused: true,
			file: &git.FileDiff{
				Path:    "new_name.go",
				OldPath: "old_name.go",
				Hunks: []git.Hunk{{
					Header: "@@ -1,1 +1,1 @@",
					Lines: []git.Line{
						{Type: git.LineContext, Content: "x", OldNum: 1, NewNum: 1},
					},
				}},
			},
			context:    3,
			hideWS:     false,
			wantTop:    "╭ old_name.go → new_name.go ─────────────────────────────────╮",
			wantBottom: "╰ Context: 3 ─────────────── +0/-0 ────────────── Whitespace ╯",
		},
		{
			name:    "large counts/width 50",
			width:   50,
			height:  8,
			focused: true,
			file: &git.FileDiff{
				Path: "big.go",
				Hunks: []git.Hunk{{
					Header: "@@ -1,768 +1,1234 @@",
					Lines:  makeLargeLineSet(1234, 768),
				}},
			},
			context:    3,
			hideWS:     false,
			wantTop:    "╭ big.go ──────────────────────────────────────────╮",
			wantBottom: "╰ Context: 3 ─────── +1234/-768 ─────── Whitespace ╯",
		},
		{
			name:    "no file selected/width 40",
			width:   40,
			height:  8,
			focused: false,
			file:    nil,
			context: 3,
			hideWS:  false,
			// No title when no file is selected
			wantTop:    "╭────────────────────────────────────────╮",
			wantBottom: "╰ Context: 3 ───── +0/-0 ──── Whitespace ╯",
		},
		{
			name:    "narrow width 20",
			width:   20,
			height:  8,
			focused: true,
			file: &git.FileDiff{
				Path: "narrow.go",
				Hunks: []git.Hunk{{
					Header: "@@ -1,1 +1,1 @@",
					Lines: []git.Line{
						{Type: git.LineAdded, Content: "a", NewNum: 1},
					},
				}},
			},
			context:    3,
			hideWS:     false,
			wantTop:    "╭ narrow.go ─────────╮",
			wantBottom: "╰ Context Whitespace ╯",
		},
		{
			name:    "non-default context/width 40",
			width:   40,
			height:  8,
			focused: true,
			file: &git.FileDiff{
				Path: "ctx.go",
				Hunks: []git.Hunk{{
					Header: "@@ -1,1 +1,1 @@",
					Lines: []git.Line{
						{Type: git.LineAdded, Content: "a", NewNum: 1},
					},
				}},
			},
			context:    10,
			hideWS:     false,
			wantTop:    "╭ ctx.go ────────────────────────────────╮",
			wantBottom: "╰ Context: 10 ──── +1/-0 ──── Whitespace ╯",
		},
		{
			name:    "whitespace hidden/width 50",
			width:   50,
			height:  8,
			focused: true,
			file: &git.FileDiff{
				Path: "ws.go",
				Hunks: []git.Hunk{{
					Header: "@@ -1,1 +1,1 @@",
					Lines: []git.Line{
						{Type: git.LineAdded, Content: "a", NewNum: 1},
					},
				}},
			},
			context:    3,
			hideWS:     true,
			wantTop:    "╭ ws.go ───────────────────────────────────────────╮",
			wantBottom: "╰ Context: 3 ────────── +1/-0 ── Whitespace hidden ╯",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newDiffViewModel()
			m.width = tt.width
			m.height = tt.height
			if tt.file != nil {
				m.setFile(tt.file)
			}
			rendered := m.render(tt.focused, tt.context, tt.hideWS)
			gotTop, gotBottom := extractBorders(rendered)

			assert.Equal(t, tt.wantTop, gotTop, "top border mismatch")
			assert.Equal(t, tt.wantBottom, gotBottom, "bottom border mismatch")
		})
	}
}

// TestFileListBorderSnapshots asserts exact top/bottom border composition
// for the file list pane.
func TestFileListBorderSnapshots(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		focused    bool
		files      []git.FileDiff
		modeSlider string
		wantTop    string
		wantBottom string
	}{
		{
			name:       "focused/with files/width 30",
			width:      30,
			height:     10,
			focused:    true,
			files:      makeFiles("a.go", "b.go", "c.go"),
			modeSlider: " Staged+Unstaged ",
			wantTop:    "╭────── Staged+Unstaged ───────╮",
			wantBottom: "╰──────────────────────────────╯",
		},
		{
			name:       "unfocused/with files/width 30",
			width:      30,
			height:     10,
			focused:    false,
			files:      makeFiles("a.go", "b.go"),
			modeSlider: " Staged+Unstaged ",
			wantTop:    "╭────── Staged+Unstaged ───────╮",
			wantBottom: "╰──────────────────────────────╯",
		},
		{
			name:       "no files/width 30",
			width:      30,
			height:     10,
			focused:    true,
			files:      nil,
			modeSlider: " Branch+Staged+Unstaged ",
			wantTop:    "╭─── Branch+Staged+Unstaged ───╮",
			wantBottom: "╰──────────────────────────────╯",
		},
		{
			name:       "narrow width 20/with files",
			width:      20,
			height:     8,
			focused:    true,
			files:      makeFiles("a.go"),
			modeSlider: " Staged ",
			wantTop:    "╭────── Staged ──────╮",
			wantBottom: "╰────────────────────╯",
		},
		{
			name:       "wide width 50/branch mode",
			width:      50,
			height:     10,
			focused:    false,
			files:      makeFiles("a.go", "b.go", "c.go", "d.go"),
			modeSlider: " Branch+Staged+Unstaged ",
			wantTop:    "╭───────────── Branch+Staged+Unstaged ─────────────╮",
			wantBottom: "╰──────────────────────────────────────────────────╯",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newFileListModel(tt.files)
			m.width = tt.width
			m.height = tt.height
			rendered := m.render(tt.focused, tt.modeSlider)
			gotTop, gotBottom := extractBorders(rendered)

			assert.Equal(t, tt.wantTop, gotTop, "top border mismatch")
			assert.Equal(t, tt.wantBottom, gotBottom, "bottom border mismatch")
		})
	}
}

// makeLargeLineSet creates a slice of Lines with the given number of added and removed lines.
func makeLargeLineSet(added, removed int) []git.Line {
	lines := make([]git.Line, 0, added+removed)
	for i := 0; i < removed; i++ {
		lines = append(lines, git.Line{Type: git.LineRemoved, Content: "old", OldNum: i + 1})
	}
	for i := 0; i < added; i++ {
		lines = append(lines, git.Line{Type: git.LineAdded, Content: "new", NewNum: i + 1})
	}
	return lines
}

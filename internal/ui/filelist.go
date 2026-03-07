package ui

import (
	"fmt"
	"strings"

	"github.com/justincampbell/revise/internal/git"
)

type fileListModel struct {
	files    []git.FileDiff
	cursor   int
	height   int
	width    int
	offset   int // scroll offset
}

func newFileListModel(files []git.FileDiff) fileListModel {
	return fileListModel{
		files: files,
	}
}

func (m *fileListModel) moveUp() {
	if m.cursor > 0 {
		m.cursor--
		m.ensureVisible()
	}
}

func (m *fileListModel) moveDown() {
	if m.cursor < len(m.files)-1 {
		m.cursor++
		m.ensureVisible()
	}
}

func (m *fileListModel) ensureVisible() {
	viewHeight := m.viewHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+viewHeight {
		m.offset = m.cursor - viewHeight + 1
	}
}

func (m *fileListModel) viewHeight() int {
	// Account for header line
	h := m.height - 2
	if h < 1 {
		h = 1
	}
	return h
}

func (m fileListModel) selectedFile() *git.FileDiff {
	if len(m.files) == 0 {
		return nil
	}
	return &m.files[m.cursor]
}

func (m fileListModel) render(focused bool) string {
	if len(m.files) == 0 {
		return "No changes"
	}

	var b strings.Builder

	// Header
	header := fmt.Sprintf(" Files (%d/%d)", m.cursor+1, len(m.files))
	b.WriteString(fileStyle.Render(header))
	b.WriteString("\n")

	viewHeight := m.viewHeight()
	end := m.offset + viewHeight
	if end > len(m.files) {
		end = len(m.files)
	}

	for i := m.offset; i < end; i++ {
		f := m.files[i]
		status := statusIndicator(f.Status)
		name := truncate(f.Path, m.width-5)

		if i == m.cursor {
			prefix := "▸ "
			b.WriteString(selectedStyle.Render(prefix + status + " " + name))
		} else {
			prefix := "  "
			b.WriteString(unselectedStyle.Render(prefix + status + " " + name))
		}

		if i < end-1 {
			b.WriteString("\n")
		}
	}

	style := panelBorder
	if focused {
		style = focusedBorder
	}
	return style.Width(m.width).Height(m.height).MaxHeight(m.height + 2).Render(b.String())
}

func statusIndicator(s git.FileStatus) string {
	switch s {
	case git.StatusModified:
		return statusModified.Render("M")
	case git.StatusAdded:
		return statusAdded.Render("A")
	case git.StatusDeleted:
		return statusDeleted.Render("D")
	case git.StatusRenamed:
		return statusModified.Render("R")
	case git.StatusUntracked:
		return statusUntracked.Render("?")
	default:
		return " "
	}
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return "…" + s[len(s)-maxLen+1:]
}

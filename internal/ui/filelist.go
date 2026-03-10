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
	comments comments
}

func newFileListModel(files []git.FileDiff) fileListModel {
	return fileListModel{
		files:    files,
		comments: make(comments),
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
	h := m.height
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

func (m fileListModel) totals() (added, removed int) {
	for _, f := range m.files {
		a, r := fileTotals(f)
		added += a
		removed += r
	}
	return added, removed
}

func fileTotals(f git.FileDiff) (added, removed int) {
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			switch l.Type {
			case git.LineAdded:
				added++
			case git.LineRemoved:
				removed++
			}
		}
	}
	return added, removed
}

func (m fileListModel) render(focused bool) string {
	style := panelBorder
	if focused {
		style = focusedBorder
	}

	if len(m.files) == 0 {
		rendered := style.Width(m.width).Height(m.height).MaxHeight(m.height + 2).Render("No changes")
		rendered = setBorderTitle(rendered, " Files ", focused)
		return rendered
	}

	var b strings.Builder

	viewHeight := m.viewHeight()
	end := m.offset + viewHeight
	if end > len(m.files) {
		end = len(m.files)
	}

	for i := m.offset; i < end; i++ {
		f := m.files[i]
		status := statusIndicator(f.Status)
		count := m.comments.countForFile(f.Path)
		countSuffix := ""
		if count > 0 {
			countSuffix = fmt.Sprintf(" (%d)", count)
		}
		name := truncate(f.Path, m.width-5-len(countSuffix))

		if i == m.cursor {
			prefix := "▸ "
			b.WriteString(selectedStyle.Render(prefix+status+" "+name) + commentCountStyle.Render(countSuffix))
		} else {
			prefix := "  "
			b.WriteString(unselectedStyle.Render(prefix+status+" "+name) + commentCountStyle.Render(countSuffix))
		}

		if i < end-1 {
			b.WriteString("\n")
		}
	}

	added, removed := m.totals()
	rendered := style.Width(m.width).Height(m.height).MaxHeight(m.height + 2).Render(b.String())

	title := fmt.Sprintf(" Files (%d/%d) ", m.cursor+1, len(m.files))
	rendered = setBorderTitle(rendered, title, focused)
	rendered = setBorderBottomCounts(rendered, added, removed, focused)
	return rendered
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

func renderPaneChangeSummary(added, removed int) string {
	summary := " " + statusAdded.Render(fmt.Sprintf("+%d", added)) + statusBarStyle.Render("/") + statusDeleted.Render(fmt.Sprintf("-%d", removed)) + " "
	return summary
}

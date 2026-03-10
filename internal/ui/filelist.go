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

	treeView bool
	tree     []*treeNode
	rows     []treeRow // flattened visible rows
}

func newFileListModel(files []git.FileDiff) fileListModel {
	return fileListModel{
		files:    files,
		comments: make(comments),
	}
}

func (m *fileListModel) toggleTreeView() {
	m.treeView = !m.treeView
	if m.treeView {
		m.rebuildTree()
	}
	// Preserve file selection when switching views
	selectedPath := ""
	if f := m.selectedFile(); f != nil {
		selectedPath = f.Path
	}
	if m.treeView && selectedPath != "" {
		for i, row := range m.rows {
			if !row.node.isDir() && row.node.fileIdx >= 0 && row.node.fileIdx < len(m.files) {
				if m.files[row.node.fileIdx].Path == selectedPath {
					m.cursor = i
					break
				}
			}
		}
	} else if !m.treeView && selectedPath != "" {
		for i, f := range m.files {
			if f.Path == selectedPath {
				m.cursor = i
				break
			}
		}
	}
	m.ensureVisible()
}

func (m *fileListModel) rebuildTree() {
	m.tree = buildTree(m.files)
	m.rows = flattenTree(m.tree)
}

func (m *fileListModel) rowCount() int {
	if m.treeView {
		return len(m.rows)
	}
	return len(m.files)
}

func (m *fileListModel) moveUp() {
	if m.cursor > 0 {
		m.cursor--
		m.ensureVisible()
	}
}

func (m *fileListModel) moveDown() {
	if m.cursor < m.rowCount()-1 {
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
	if m.treeView {
		if m.cursor < 0 || m.cursor >= len(m.rows) {
			return nil
		}
		row := m.rows[m.cursor]
		if row.node.isDir() {
			return nil
		}
		if row.node.fileIdx >= 0 && row.node.fileIdx < len(m.files) {
			return &m.files[row.node.fileIdx]
		}
		return nil
	}
	if len(m.files) == 0 {
		return nil
	}
	return &m.files[m.cursor]
}

// toggleExpand toggles expand/collapse on a directory node in tree view.
// Returns true if the current row is a directory.
func (m *fileListModel) toggleExpand() bool {
	if !m.treeView || m.cursor < 0 || m.cursor >= len(m.rows) {
		return false
	}
	row := m.rows[m.cursor]
	if !row.node.isDir() {
		return false
	}
	row.node.expanded = !row.node.expanded
	m.rows = flattenTree(m.tree)
	// Clamp cursor if rows shrunk
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	m.ensureVisible()
	return true
}

// expandNode expands the current directory node.
// Returns true if it handled the action (current row is a collapsed dir).
func (m *fileListModel) expandNode() bool {
	if !m.treeView || m.cursor < 0 || m.cursor >= len(m.rows) {
		return false
	}
	row := m.rows[m.cursor]
	if !row.node.isDir() {
		return false
	}
	if row.node.expanded {
		return false
	}
	row.node.expanded = true
	m.rows = flattenTree(m.tree)
	m.ensureVisible()
	return true
}

// collapseOrParent collapses the current directory if expanded,
// or moves cursor to the parent directory. Returns true if handled.
func (m *fileListModel) collapseOrParent() bool {
	if !m.treeView || m.cursor < 0 || m.cursor >= len(m.rows) {
		return false
	}
	row := m.rows[m.cursor]

	// If on an expanded directory, collapse it
	if row.node.isDir() && row.node.expanded {
		row.node.expanded = false
		m.rows = flattenTree(m.tree)
		if m.cursor >= len(m.rows) {
			m.cursor = len(m.rows) - 1
		}
		m.ensureVisible()
		return true
	}

	// Otherwise, move to parent directory
	parent := row.node.parent
	if parent == nil {
		return false
	}
	for i, r := range m.rows {
		if r.node == parent {
			m.cursor = i
			m.ensureVisible()
			return true
		}
	}
	return false
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

	if m.treeView {
		m.renderTreeRows(&b)
	} else {
		m.renderFlatRows(&b)
	}

	added, removed := m.totals()
	rendered := style.Width(m.width).Height(m.height).MaxHeight(m.height + 2).Render(b.String())

	fileNum := m.selectedFileNum()
	title := fmt.Sprintf(" Files (%d/%d) ", fileNum, len(m.files))
	rendered = setBorderTitle(rendered, title, focused)
	rendered = setBorderBottomCounts(rendered, added, removed, focused)
	return rendered
}

func (m fileListModel) selectedFileNum() int {
	if m.treeView {
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			row := m.rows[m.cursor]
			if !row.node.isDir() && row.node.fileIdx >= 0 {
				return row.node.fileIdx + 1
			}
		}
		// On a directory, show 0
		return 0
	}
	return m.cursor + 1
}

func (m fileListModel) renderFlatRows(b *strings.Builder) {
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
}

func (m fileListModel) renderTreeRows(b *strings.Builder) {
	viewHeight := m.viewHeight()
	end := m.offset + viewHeight
	if end > len(m.rows) {
		end = len(m.rows)
	}

	for i := m.offset; i < end; i++ {
		row := m.rows[i]
		prefix := row.prefix
		prefixWidth := len([]rune(prefix))
		availWidth := m.width - 2 - prefixWidth // -2 for border padding

		if row.node.isDir() {
			label := treeLabel(row)
			label = truncate(label, availWidth)
			if i == m.cursor {
				b.WriteString(selectedStyle.Render(prefix + label))
			} else {
				b.WriteString(dirStyle.Render(prefix + label))
			}
		} else {
			status := treeFileStatus(row, m.files)
			statusStr := statusIndicator(status)
			filePath := treeFilePath(row, m.files)
			count := m.comments.countForFile(filePath)
			countSuffix := ""
			if count > 0 {
				countSuffix = fmt.Sprintf(" (%d)", count)
			}
			// prefix + status + " " + name + countSuffix must fit
			name := truncate(row.node.name, availWidth-2-len(countSuffix))

			if i == m.cursor {
				b.WriteString(selectedStyle.Render(prefix+statusStr+" "+name) + commentCountStyle.Render(countSuffix))
			} else {
				b.WriteString(unselectedStyle.Render(prefix+statusStr+" "+name) + commentCountStyle.Render(countSuffix))
			}
		}

		if i < end-1 {
			b.WriteString("\n")
		}
	}
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

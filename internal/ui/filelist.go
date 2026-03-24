package ui

import (
	"fmt"
	"strings"

	"github.com/justincampbell/revise/internal/git"
)

// treeRow represents a single row in the tree view — either a directory header or a file.
type treeRow struct {
	name    string // display name (just the basename or dir name with trailing /)
	depth   int    // indentation level
	isDir   bool   // true for directory headers
	fileIdx int    // index into files slice (-1 for dirs)
}

type fileListModel struct {
	files    []git.FileDiff
	cursor   int
	height   int
	width    int
	offset   int // scroll offset
	comments comments
	treeView bool      // toggle between flat and tree view
	treeRows []treeRow // cached tree rows, rebuilt when files change or tree toggled
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
		m.rebuildTreeRows()
	}
}

func (m *fileListModel) rebuildTreeRows() {
	m.treeRows = buildTreeRows(m.files)
}

// treeRowForFile returns the tree row index for the given file index.
func (m *fileListModel) treeRowForFile(fileIdx int) int {
	for i, r := range m.treeRows {
		if !r.isDir && r.fileIdx == fileIdx {
			return i
		}
	}
	return 0
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
	if m.treeView && len(m.treeRows) > 0 {
		// In tree view, offset is in tree row indices
		treeIdx := m.treeRowForFile(m.cursor)
		if treeIdx < m.offset {
			m.offset = treeIdx
		}
		if treeIdx >= m.offset+viewHeight {
			m.offset = treeIdx - viewHeight + 1
		}
	} else {
		if m.cursor < m.offset {
			m.offset = m.cursor
		}
		if m.cursor >= m.offset+viewHeight {
			m.offset = m.cursor - viewHeight + 1
		}
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

func (m fileListModel) render(focused bool, modeSlider string) string {
	style := panelBorder
	if focused {
		style = focusedBorder
	}

	if len(m.files) == 0 {
		rendered := style.Width(m.width).Height(m.height).MaxHeight(m.height + 2).Render("No changes")
		rendered = setBorderTitleCentered(rendered, modeSlider, focused)
		return rendered
	}

	var b strings.Builder

	if m.treeView && len(m.treeRows) > 0 {
		m.renderTree(&b)
	} else {
		m.renderFlat(&b)
	}

	rendered := style.Width(m.width).Height(m.height).MaxHeight(m.height + 2).Render(b.String())

	rendered = setBorderTitleCentered(rendered, modeSlider, focused)
	return rendered
}

func (m fileListModel) renderFlat(b *strings.Builder) {
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

func (m fileListModel) renderTree(b *strings.Builder) {
	viewHeight := m.viewHeight()
	end := m.offset + viewHeight
	if end > len(m.treeRows) {
		end = len(m.treeRows)
	}

	for i := m.offset; i < end; i++ {
		row := m.treeRows[i]
		indent := strings.Repeat("  ", row.depth)

		if row.isDir {
			// Directory header
			b.WriteString(unselectedStyle.Render("  " + indent + row.name))
		} else {
			f := m.files[row.fileIdx]
			status := statusIndicator(f.Status)
			count := m.comments.countForFile(f.Path)
			countSuffix := ""
			if count > 0 {
				countSuffix = fmt.Sprintf(" (%d)", count)
			}
			// Available width: total width - prefix(2) - indent - status(1) - space(1) - countSuffix
			availWidth := m.width - 4 - len(indent) - len(countSuffix)
			name := truncate(row.name, availWidth)

			if row.fileIdx == m.cursor {
				prefix := "▸ "
				b.WriteString(selectedStyle.Render(prefix+indent+status+" "+name) + commentCountStyle.Render(countSuffix))
			} else {
				prefix := "  "
				b.WriteString(unselectedStyle.Render(prefix+indent+status+" "+name) + commentCountStyle.Render(countSuffix))
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

// buildTreeRows creates a flat list of tree rows from the file list,
// grouping files by directory with indentation.
func buildTreeRows(files []git.FileDiff) []treeRow {
	if len(files) == 0 {
		return nil
	}

	type dirEntry struct {
		name    string
		depth   int
		fileIdx int // -1 for directory
	}

	var rows []treeRow
	// Track which directory path segments have been emitted at each depth
	var currentDirs []string // current directory path segments

	for fileIdx, f := range files {
		parts := strings.Split(f.Path, "/")
		dirParts := parts[:len(parts)-1]
		fileName := parts[len(parts)-1]

		// Find how many leading directory segments match
		commonDepth := 0
		for commonDepth < len(currentDirs) && commonDepth < len(dirParts) {
			if currentDirs[commonDepth] != dirParts[commonDepth] {
				break
			}
			commonDepth++
		}

		// Emit new directory rows for segments beyond the common prefix
		for d := commonDepth; d < len(dirParts); d++ {
			rows = append(rows, treeRow{
				name:    dirParts[d] + "/",
				depth:   d,
				isDir:   true,
				fileIdx: -1,
			})
		}
		currentDirs = dirParts

		// Emit the file row
		rows = append(rows, treeRow{
			name:    fileName,
			depth:   len(dirParts),
			isDir:   false,
			fileIdx: fileIdx,
		})
	}

	return rows
}

func renderPaneChangeSummary(added, removed int) string {
	summary := " " + statusAdded.Render(fmt.Sprintf("+%d", added)) + statusBarStyle.Render("/") + statusDeleted.Render(fmt.Sprintf("-%d", removed)) + " "
	return summary
}

package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/justincampbell/revise/internal/git"
)

type fileListModel struct {
	files    []git.FileDiff
	cursor   int
	height   int
	width    int
	offset   int // scroll offset
	comments comments
	marks    marks

	// Branch-mode commit list, rendered as a section above the files.
	commits     []git.CommitInfo // newest first
	branchDepth int              // 0 = full branch; N = last N commits in scope
	showCommits bool             // true in Branch mode with commits to show
}

// commitsMinFileRows is how many file rows the commit section tries to leave
// visible when it would otherwise grow tall enough to crowd out the files.
const commitsMinFileRows = 3

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
	h := m.height - m.commitsSectionHeight()
	if h < 1 {
		h = 1
	}
	return h
}

// effectiveDepth is the number of commits in scope: branchDepth, or all of them
// when full (0) or out of range.
func (m fileListModel) effectiveDepth() int {
	if m.branchDepth <= 0 || m.branchDepth > len(m.commits) {
		return len(m.commits)
	}
	return m.branchDepth
}

// commitRowsShown is how many commit rows the section renders, capped so the
// file list keeps at least a few rows. When capped, the last row is a
// "… +N more" summary.
func (m fileListModel) commitRowsShown() int {
	n := len(m.commits)
	if n == 0 {
		return 0
	}
	budget := m.height - 2 - commitsMinFileRows // minus header + separator
	if budget < 1 {
		budget = 1
	}
	if n <= budget {
		return n
	}
	return budget
}

// commitsSectionHeight is the total rows the commit section occupies: header +
// commit rows + separator (0 when not shown).
func (m fileListModel) commitsSectionHeight() int {
	if !m.showCommits || len(m.commits) == 0 {
		return 0
	}
	return m.commitRowsShown() + 2
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

	innerWidth := m.width - 2 // subtract left+right border

	var lines []string
	// Branch-mode commit list, above the files.
	lines = append(lines, m.renderCommitsLines(innerWidth)...)

	if len(m.files) == 0 {
		lines = append(lines, unselectedStyle.Render("No changes"))
	} else {
		viewHeight := m.viewHeight()
		end := m.offset + viewHeight
		if end > len(m.files) {
			end = len(m.files)
		}

		for i := m.offset; i < end; i++ {
			f := m.files[i]
			status := statusIndicator(f.Status, fileStagingSources(f))
			commentCount := m.comments.countForFile(f.Path)
			markCount := m.marks.countForFile(f.Path)
			commentSuffix := ""
			markSuffix := ""
			if commentCount > 0 {
				commentSuffix = fmt.Sprintf(" (%d)", commentCount)
			}
			if markCount > 0 {
				markSuffix = fmt.Sprintf(" (%d)", markCount)
			}
			suffixLen := len(commentSuffix) + len(markSuffix)
			name := truncate(f.Path, m.width-5-suffixLen)

			var row string
			if i == m.cursor {
				prefix := "▸ "
				row = selectedStyle.Render(prefix) + status + selectedStyle.Render(" "+name) + commentCountStyle.Render(commentSuffix) + markCountStyle.Render(markSuffix)
			} else {
				prefix := "  "
				row = unselectedStyle.Render(prefix) + status + unselectedStyle.Render(" "+name) + commentCountStyle.Render(commentSuffix) + markCountStyle.Render(markSuffix)
			}
			lines = append(lines, ansi.Truncate(row, innerWidth, ""))
		}
	}

	// m.height is the inner content height; lipgloss v2 includes borders in
	// Height(), so we pass m.height + 2 (top + bottom border).
	rendered := style.Width(m.width).Height(m.height + 2).MaxHeight(m.height + 2).Render(strings.Join(lines, "\n"))

	rendered = setBorderTitleCentered(rendered, modeSlider, focused)
	return rendered
}

// renderCommitsLines builds the Branch-mode "Commits" section: a header, one
// row per commit (● in scope, · excluded), and a separator. Returns nil when
// not in Branch mode or there are no commits.
func (m fileListModel) renderCommitsLines(innerWidth int) []string {
	if !m.showCommits || len(m.commits) == 0 {
		return nil
	}

	total := len(m.commits)
	eff := m.effectiveDepth()
	rows := m.commitRowsShown()

	// The section only shows while filtering, so the header always describes a
	// "last N" selection (never the full branch).
	label := fmt.Sprintf("Last %d commits", eff)
	if eff == 1 {
		label = "Last commit"
	}

	lines := []string{ansi.Truncate(commitHeaderStyle.Render(label), innerWidth, "")}

	truncated := rows < total
	for i := 0; i < rows; i++ {
		if truncated && i == rows-1 {
			hidden := total - i // remaining commits, including this slot
			lines = append(lines, commitExcludedStyle.Render(fmt.Sprintf("  … +%d more", hidden)))
			break
		}
		lines = append(lines, m.renderCommitRow(m.commits[i], i < eff, innerWidth))
	}

	lines = append(lines, commitHeaderStyle.Render(strings.Repeat("─", innerWidth)))
	return lines
}

// renderCommitRow formats one commit as "<marker> <subject>". In-scope commits
// are bright with a ● marker; excluded ones are dimmed with a · marker.
func (m fileListModel) renderCommitRow(c git.CommitInfo, inScope bool, innerWidth int) string {
	marker := "·"
	mStyle := commitExcludedStyle
	tStyle := commitExcludedStyle
	if inScope {
		marker = "●"
		mStyle = commitMarkerStyle
		tStyle = commitInScopeStyle
	}

	subjMax := innerWidth - 2 // marker + space
	if subjMax < 1 {
		subjMax = 1
	}
	subj := ansi.Truncate(c.Subject, subjMax, "…")

	row := mStyle.Render(marker) + " " + tStyle.Render(subj)
	return ansi.Truncate(row, innerWidth, "")
}

type stagingSources struct {
	branch   bool
	staged   bool
	unstaged bool
}

func fileStagingSources(f git.FileDiff) stagingSources {
	var s stagingSources
	for _, h := range f.Hunks {
		switch h.Source {
		case git.SourceBranch:
			s.branch = true
		case git.SourceStaged:
			s.staged = true
		case git.SourceUnstaged:
			s.unstaged = true
		}
	}
	return s
}

func statusIndicator(s git.FileStatus, staging stagingSources) string {
	letter := statusLetter(s)
	style := statusStyle(s, staging)
	return style.Render(letter)
}

func statusLetter(s git.FileStatus) string {
	switch s {
	case git.StatusModified:
		return "M"
	case git.StatusAdded:
		return "A"
	case git.StatusDeleted:
		return "D"
	case git.StatusRenamed:
		return "R"
	case git.StatusUntracked:
		return "?"
	default:
		return " "
	}
}

func statusStyle(s git.FileStatus, staging stagingSources) lipgloss.Style {
	// Partially staged: cyan
	if staging.staged && staging.unstaged {
		return statusPartiallyStaged
	}
	// Fully staged: green
	if staging.staged {
		return statusAdded
	}
	// Branch only (committed, no working tree changes): dim variant of status color
	if staging.branch && !staging.unstaged {
		switch s {
		case git.StatusModified:
			return statusDimModified
		case git.StatusAdded:
			return statusDimAdded
		case git.StatusDeleted:
			return statusDimDeleted
		case git.StatusRenamed:
			return statusDimRenamed
		default:
			return lipgloss.NewStyle()
		}
	}
	// Unstaged or default: use the file status color
	switch s {
	case git.StatusModified:
		return statusModified
	case git.StatusAdded:
		return statusAdded
	case git.StatusDeleted:
		return statusDeleted
	case git.StatusRenamed:
		return statusModified
	case git.StatusUntracked:
		return statusUntracked
	default:
		return lipgloss.NewStyle()
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

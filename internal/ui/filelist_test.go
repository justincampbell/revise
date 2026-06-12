package ui

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/justincampbell/revise/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeFiles(paths ...string) []git.FileDiff {
	files := make([]git.FileDiff, len(paths))
	for i, p := range paths {
		files[i] = git.FileDiff{Path: p, Status: git.StatusModified}
	}
	return files
}

func TestFileListMoveDown_Clamps(t *testing.T) {
	m := newFileListModel(makeFiles("a", "b"))
	m.moveDown()
	m.moveDown() // should not go past index 1
	assert.Equal(t, 1, m.cursor)
}

func TestFileListMoveUp_Clamps(t *testing.T) {
	m := newFileListModel(makeFiles("a", "b"))
	m.moveUp()
	assert.Equal(t, 0, m.cursor)
}

func TestFileListMoveDown_ThenUp(t *testing.T) {
	m := newFileListModel(makeFiles("a", "b", "c"))
	m.moveDown()
	m.moveDown()
	m.moveUp()
	assert.Equal(t, 1, m.cursor)
}

func TestFileListSelectedFile(t *testing.T) {
	m := newFileListModel(makeFiles("a.go", "b.go"))
	m.moveDown()
	f := m.selectedFile()
	require.NotNil(t, f)
	assert.Equal(t, "b.go", f.Path)
}

func TestFileListSelectedFile_Empty(t *testing.T) {
	m := newFileListModel(nil)
	assert.Nil(t, m.selectedFile())
}

func TestFileListEnsureVisible_ScrollsDown(t *testing.T) {
	m := newFileListModel(makeFiles("a", "b", "c", "d", "e"))
	m.height = 3 // viewHeight = 3
	m.cursor = 4
	m.ensureVisible()
	assert.Equal(t, 2, m.offset)
}

func TestFileListEnsureVisible_ScrollsUp(t *testing.T) {
	m := newFileListModel(makeFiles("a", "b", "c", "d", "e"))
	m.height = 5 // viewHeight = 5
	m.offset = 3
	m.cursor = 1
	m.ensureVisible()
	assert.Equal(t, 1, m.offset)
}

func TestTruncate_Short(t *testing.T) {
	assert.Equal(t, "abc", truncate("abc", 10))
}

func TestTruncate_Long(t *testing.T) {
	got := truncate("abcdefghij", 6)
	assert.Equal(t, 6, len([]rune(got)), "should be 6 runes")
	assert.Contains(t, got, "…")
}

func TestTruncate_ZeroMax(t *testing.T) {
	assert.Equal(t, "abc", truncate("abc", 0))
}

func TestTruncate_VerySmallMax(t *testing.T) {
	assert.Equal(t, "ab", truncate("abcdef", 2))
}

func TestFileList_ModeSliderInBorder(t *testing.T) {
	m := newFileListModel(makeFiles("a.go", "b.go", "c.go"))
	m.width = 30
	m.height = 10
	rendered := m.render(true, "Staged+Unstaged")
	assert.Contains(t, rendered, "Staged+Unstaged")
}

func TestFileList_NoContextInFooter(t *testing.T) {
	m := newFileListModel(makeFiles("a.go"))
	m.width = 40
	m.height = 8
	rendered := m.render(true, "Staged")
	assert.NotContains(t, rendered, "Context")
}

func TestFileTotals(t *testing.T) {
	f := git.FileDiff{
		Path:   "a.go",
		Status: git.StatusModified,
		Hunks: []git.Hunk{{
			Header: "@@ -1,1 +1,2 @@",
			Lines: []git.Line{
				{Type: git.LineAdded, Content: "a", NewNum: 1},
				{Type: git.LineRemoved, Content: "b", OldNum: 1},
				{Type: git.LineAdded, Content: "c", NewNum: 2},
			},
		}},
	}
	added, removed := fileTotals(f)
	assert.Equal(t, 2, added)
	assert.Equal(t, 1, removed)
}

func TestFileStagingSources(t *testing.T) {
	tests := []struct {
		name  string
		hunks []git.Hunk
		want  stagingSources
	}{
		{
			name:  "staged only",
			hunks: []git.Hunk{{Source: git.SourceStaged}},
			want:  stagingSources{staged: true},
		},
		{
			name:  "unstaged only",
			hunks: []git.Hunk{{Source: git.SourceUnstaged}},
			want:  stagingSources{unstaged: true},
		},
		{
			name:  "branch only",
			hunks: []git.Hunk{{Source: git.SourceBranch}},
			want:  stagingSources{branch: true},
		},
		{
			name: "partially staged",
			hunks: []git.Hunk{
				{Source: git.SourceStaged},
				{Source: git.SourceUnstaged},
			},
			want: stagingSources{staged: true, unstaged: true},
		},
		{
			name: "all sources",
			hunks: []git.Hunk{
				{Source: git.SourceBranch},
				{Source: git.SourceStaged},
				{Source: git.SourceUnstaged},
			},
			want: stagingSources{branch: true, staged: true, unstaged: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := git.FileDiff{Hunks: tt.hunks}
			got := fileStagingSources(f)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStatusIndicator(t *testing.T) {
	tests := []struct {
		name    string
		status  git.FileStatus
		staging stagingSources
		want    string // expected rendered string (letter + styling)
	}{
		// Fully staged: green style applied to the status letter
		{"modified fully staged", git.StatusModified, stagingSources{staged: true}, statusAdded.Render("M")},
		{"added fully staged", git.StatusAdded, stagingSources{staged: true}, statusAdded.Render("A")},
		{"deleted fully staged", git.StatusDeleted, stagingSources{staged: true}, statusAdded.Render("D")},

		// Partially staged: cyan style
		{"modified partially staged", git.StatusModified, stagingSources{staged: true, unstaged: true}, statusPartiallyStaged.Render("M")},
		{"added partially staged", git.StatusAdded, stagingSources{staged: true, unstaged: true}, statusPartiallyStaged.Render("A")},

		// Unstaged only: default file status color
		{"modified unstaged", git.StatusModified, stagingSources{unstaged: true}, statusModified.Render("M")},
		{"added unstaged", git.StatusAdded, stagingSources{unstaged: true}, statusAdded.Render("A")},
		{"deleted unstaged", git.StatusDeleted, stagingSources{unstaged: true}, statusDeleted.Render("D")},
		{"untracked", git.StatusUntracked, stagingSources{unstaged: true}, statusUntracked.Render("?")},

		// Branch only: dim variant of status color
		{"modified branch", git.StatusModified, stagingSources{branch: true}, statusDimModified.Render("M")},
		{"added branch", git.StatusAdded, stagingSources{branch: true}, statusDimAdded.Render("A")},
		{"deleted branch", git.StatusDeleted, stagingSources{branch: true}, statusDimDeleted.Render("D")},
		{"renamed branch", git.StatusRenamed, stagingSources{branch: true}, statusDimRenamed.Render("R")},

		// Branch+unstaged: unstaged color (working tree changes take priority)
		{"modified branch+unstaged", git.StatusModified, stagingSources{branch: true, unstaged: true}, statusModified.Render("M")},

		// Branch+staged: green
		{"modified branch+staged", git.StatusModified, stagingSources{branch: true, staged: true}, statusAdded.Render("M")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statusIndicator(tt.status, tt.staging)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStatusLetter(t *testing.T) {
	tests := []struct {
		status git.FileStatus
		want   string
	}{
		{git.StatusModified, "M"},
		{git.StatusAdded, "A"},
		{git.StatusDeleted, "D"},
		{git.StatusRenamed, "R"},
		{git.StatusUntracked, "?"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, statusLetter(tt.status))
		})
	}
}

func TestFileListTotals(t *testing.T) {
	m := newFileListModel([]git.FileDiff{
		{
			Path:   "a.go",
			Status: git.StatusModified,
			Hunks: []git.Hunk{{
				Header: "@@ -1,1 +1,2 @@",
				Lines: []git.Line{
					{Type: git.LineAdded, Content: "a", NewNum: 1},
					{Type: git.LineRemoved, Content: "b", OldNum: 1},
				},
			}},
		},
		{
			Path:   "b.go",
			Status: git.StatusModified,
			Hunks: []git.Hunk{{
				Header: "@@ -1,0 +1,1 @@",
				Lines: []git.Line{
					{Type: git.LineAdded, Content: "c", NewNum: 1},
				},
			}},
		},
	})
	added, removed := m.totals()
	assert.Equal(t, 2, added)
	assert.Equal(t, 1, removed)
}

// commitList builds n dummy commits, newest first.
func commitList(n int) []git.CommitInfo {
	cs := make([]git.CommitInfo, n)
	for i := 0; i < n; i++ {
		cs[i] = git.CommitInfo{
			SHA:      fmt.Sprintf("sha%d", n-i),
			ShortSHA: fmt.Sprintf("s%d", n-i),
			Subject:  fmt.Sprintf("commit %d", n-i),
		}
	}
	return cs
}

func TestCommitsSection_HiddenWhenNotBranchMode(t *testing.T) {
	m := fileListModel{files: makeFiles("a.go"), height: 20, width: 30}
	m.commits = commitList(3)
	m.showCommits = false
	assert.Equal(t, 0, m.commitsSectionHeight())
	assert.Equal(t, 20, m.viewHeight(), "no commit section means files get the full height")
}

func TestCommitsSection_HeightAndViewportReservation(t *testing.T) {
	m := fileListModel{files: makeFiles("a.go"), height: 20, width: 30}
	m.commits = commitList(4)
	m.showCommits = true
	// header + 4 commit rows + separator = 6
	assert.Equal(t, 6, m.commitsSectionHeight())
	assert.Equal(t, 14, m.viewHeight(), "file viewport shrinks by the commit section height")
}

func TestCommitsSection_EffectiveDepth(t *testing.T) {
	m := fileListModel{height: 20, width: 30, showCommits: true, commits: commitList(5)}

	m.branchDepth = 0 // full
	assert.Equal(t, 5, m.effectiveDepth())

	m.branchDepth = 2
	assert.Equal(t, 2, m.effectiveDepth())

	m.branchDepth = 99 // out of range clamps to all
	assert.Equal(t, 5, m.effectiveDepth())
}

func TestCommitsSection_CapsRowsAndSummarizesOverflow(t *testing.T) {
	// A short pane with many commits: rows are capped, leaving file rows.
	m := fileListModel{files: makeFiles("a.go"), height: 10, width: 40, showCommits: true}
	m.commits = commitList(30)

	rows := m.commitRowsShown()
	// budget = height - 2 - commitsMinFileRows = 10 - 2 - 3 = 5
	assert.Equal(t, 5, rows)
	assert.Less(t, rows, len(m.commits), "capped below the total")

	// Section height = rows + header + separator; viewport keeps the reserve.
	assert.Equal(t, 7, m.commitsSectionHeight())
	assert.GreaterOrEqual(t, m.viewHeight(), commitsMinFileRows)

	// The rendered section shows the cap with a "more" summary and the header.
	m.branchDepth = 29 // last all-but-one (max filter depth for 30 commits)
	lines := m.renderCommitsLines(m.width - 2)
	require.Len(t, lines, 7) // header + 5 rows + separator
	assert.Contains(t, ansi.Strip(lines[0]), "Last 29 commits")
	assert.Contains(t, ansi.Strip(lines[5]), "more", "overflow summarized on the last commit row")
}

func TestCommitsSection_InScopeMarkers(t *testing.T) {
	m := fileListModel{files: makeFiles("a.go"), height: 20, width: 40, showCommits: true}
	m.commits = commitList(4)
	m.branchDepth = 2 // last 2 in scope

	lines := m.renderCommitsLines(m.width - 2)
	require.Len(t, lines, 6) // header + 4 + separator
	// commits[0], commits[1] in scope (●); commits[2], commits[3] excluded (·)
	assert.Contains(t, ansi.Strip(lines[1]), "●")
	assert.Contains(t, ansi.Strip(lines[2]), "●")
	assert.Contains(t, ansi.Strip(lines[3]), "·")
	assert.Contains(t, ansi.Strip(lines[4]), "·")
	assert.Contains(t, ansi.Strip(lines[0]), "Last 2 commits")
}

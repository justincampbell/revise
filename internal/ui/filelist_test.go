package ui

import (
	"testing"

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

func TestFileList_TitleInBorder(t *testing.T) {
	m := newFileListModel(makeFiles("a.go", "b.go", "c.go"))
	m.width = 30
	m.height = 10
	rendered := m.render(true)
	assert.Contains(t, rendered, "Files (1/3)")
}

func TestFileList_RenderShowsCenteredFooterTotals(t *testing.T) {
	m := newFileListModel([]git.FileDiff{
		{
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
		},
	})
	m.width = 40
	m.height = 8
	rendered := m.render(true)
	assert.Contains(t, rendered, "+2/-1")
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

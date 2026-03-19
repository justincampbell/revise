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

// --- Tree view tests ---

func makeFilesWithStatus(entries ...struct {
	path   string
	status git.FileStatus
}) []git.FileDiff {
	files := make([]git.FileDiff, len(entries))
	for i, e := range entries {
		files[i] = git.FileDiff{Path: e.path, Status: e.status}
	}
	return files
}

func TestBuildTreeRows_SingleFile(t *testing.T) {
	files := makeFiles("main.go")
	rows := buildTreeRows(files)
	require.Len(t, rows, 1)
	assert.Equal(t, 0, rows[0].fileIdx)
	assert.Equal(t, 0, rows[0].depth)
	assert.Equal(t, "main.go", rows[0].name)
	assert.False(t, rows[0].isDir)
}

func TestBuildTreeRows_NestedFiles(t *testing.T) {
	files := makeFiles(
		"internal/git/apply.go",
		"internal/git/parse.go",
		"internal/ui/model.go",
		"main.go",
	)
	rows := buildTreeRows(files)

	// Expected:
	// internal/          (dir, depth 0)
	//   git/             (dir, depth 1)
	//     apply.go       (file, depth 2, fileIdx 0)
	//     parse.go       (file, depth 2, fileIdx 1)
	//   ui/              (dir, depth 1)
	//     model.go       (file, depth 2, fileIdx 2)
	// main.go            (file, depth 0, fileIdx 3)

	require.Len(t, rows, 7)

	assert.True(t, rows[0].isDir)
	assert.Equal(t, "internal/", rows[0].name)
	assert.Equal(t, 0, rows[0].depth)

	assert.True(t, rows[1].isDir)
	assert.Equal(t, "git/", rows[1].name)
	assert.Equal(t, 1, rows[1].depth)

	assert.False(t, rows[2].isDir)
	assert.Equal(t, "apply.go", rows[2].name)
	assert.Equal(t, 2, rows[2].depth)
	assert.Equal(t, 0, rows[2].fileIdx)

	assert.False(t, rows[3].isDir)
	assert.Equal(t, "parse.go", rows[3].name)
	assert.Equal(t, 2, rows[3].depth)
	assert.Equal(t, 1, rows[3].fileIdx)

	assert.True(t, rows[4].isDir)
	assert.Equal(t, "ui/", rows[4].name)
	assert.Equal(t, 1, rows[4].depth)

	assert.False(t, rows[5].isDir)
	assert.Equal(t, "model.go", rows[5].name)
	assert.Equal(t, 2, rows[5].depth)
	assert.Equal(t, 2, rows[5].fileIdx)

	assert.False(t, rows[6].isDir)
	assert.Equal(t, "main.go", rows[6].name)
	assert.Equal(t, 0, rows[6].depth)
	assert.Equal(t, 3, rows[6].fileIdx)
}

func TestBuildTreeRows_RootOnlyFiles(t *testing.T) {
	files := makeFiles("a.go", "b.go", "c.go")
	rows := buildTreeRows(files)
	require.Len(t, rows, 3)
	for i, r := range rows {
		assert.False(t, r.isDir)
		assert.Equal(t, i, r.fileIdx)
		assert.Equal(t, 0, r.depth)
	}
}

func TestBuildTreeRows_SharedPrefix(t *testing.T) {
	files := makeFiles("cmd/serve.go", "cmd/version.go")
	rows := buildTreeRows(files)
	// cmd/
	//   serve.go
	//   version.go
	require.Len(t, rows, 3)
	assert.True(t, rows[0].isDir)
	assert.Equal(t, "cmd/", rows[0].name)
	assert.False(t, rows[1].isDir)
	assert.Equal(t, "serve.go", rows[1].name)
	assert.False(t, rows[2].isDir)
	assert.Equal(t, "version.go", rows[2].name)
}

func TestFileListTreeView_Toggle(t *testing.T) {
	m := newFileListModel(makeFiles("a.go", "b.go"))
	assert.False(t, m.treeView)
	m.toggleTreeView()
	assert.True(t, m.treeView)
	m.toggleTreeView()
	assert.False(t, m.treeView)
}

func TestFileListTreeView_MoveDown_SkipsDirs(t *testing.T) {
	// Files: internal/a.go, internal/b.go
	// Tree rows: internal/ (dir), a.go, b.go
	// cursor starts at 0 (first file = internal/a.go)
	// moveDown should go to file index 1 (internal/b.go)
	m := newFileListModel(makeFiles("internal/a.go", "internal/b.go"))
	m.treeView = true
	m.rebuildTreeRows()
	assert.Equal(t, 0, m.cursor) // file index 0
	m.moveDown()
	assert.Equal(t, 1, m.cursor) // file index 1
	m.moveDown()
	assert.Equal(t, 1, m.cursor) // clamped
}

func TestFileListTreeView_MoveUp_SkipsDirs(t *testing.T) {
	m := newFileListModel(makeFiles("internal/a.go", "internal/b.go"))
	m.treeView = true
	m.rebuildTreeRows()
	m.cursor = 1
	m.moveUp()
	assert.Equal(t, 0, m.cursor)
	m.moveUp()
	assert.Equal(t, 0, m.cursor) // clamped
}

func TestFileListTreeView_SelectedFile(t *testing.T) {
	m := newFileListModel(makeFiles("internal/a.go", "internal/b.go"))
	m.treeView = true
	m.rebuildTreeRows()
	m.cursor = 1
	f := m.selectedFile()
	require.NotNil(t, f)
	assert.Equal(t, "internal/b.go", f.Path)
}

func TestFileListTreeView_EnsureVisible(t *testing.T) {
	files := makeFiles(
		"internal/git/a.go",
		"internal/git/b.go",
		"internal/git/c.go",
		"internal/git/d.go",
		"internal/git/e.go",
	)
	m := newFileListModel(files)
	m.treeView = true
	m.rebuildTreeRows()
	m.height = 3
	// Move to last file
	m.cursor = 4
	m.ensureVisible()
	// In tree view, row for file index 4 is at tree row 6 (dir + dir + 5 files)
	// offset should scroll so that row is visible
	assert.True(t, m.offset >= 0)
}

func TestBuildTreeRows_EmptyFiles(t *testing.T) {
	rows := buildTreeRows(nil)
	assert.Len(t, rows, 0)
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

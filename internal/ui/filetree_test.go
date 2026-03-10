package ui

import (
	"testing"

	"github.com/justincampbell/revise/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeFileDiffs(paths ...string) []git.FileDiff {
	files := make([]git.FileDiff, len(paths))
	for i, p := range paths {
		files[i] = git.FileDiff{Path: p, Status: git.StatusModified}
	}
	return files
}

func TestBuildTree_FlatFiles(t *testing.T) {
	files := makeFileDiffs("a.go", "b.go", "c.go")
	tree := buildTree(files)

	// All files at root level, no directories
	assert.Len(t, tree, 3)
	for _, n := range tree {
		assert.False(t, n.isDir())
	}
	assert.Equal(t, "a.go", tree[0].name)
	assert.Equal(t, "b.go", tree[1].name)
	assert.Equal(t, "c.go", tree[2].name)
}

func TestBuildTree_NestedFiles(t *testing.T) {
	files := makeFileDiffs(
		"internal/ui/model.go",
		"internal/ui/filelist.go",
		"internal/git/diff.go",
		"main.go",
	)
	tree := buildTree(files)

	// Should have: internal/ dir and main.go file
	// Directories sort first
	require.Len(t, tree, 2)
	assert.True(t, tree[0].isDir())
	assert.Equal(t, "main.go", tree[1].name)
}

func TestBuildTree_CollapsesChains(t *testing.T) {
	files := makeFileDiffs(
		"internal/ui/model.go",
		"internal/ui/filelist.go",
	)
	tree := buildTree(files)

	// "internal" has one child "ui" → collapsed to "internal/ui"
	require.Len(t, tree, 1)
	assert.True(t, tree[0].isDir())
	assert.Equal(t, "internal/ui", tree[0].name)
	assert.Len(t, tree[0].children, 2)
}

func TestBuildTree_NoCollapseWithMultipleChildren(t *testing.T) {
	files := makeFileDiffs(
		"internal/ui/model.go",
		"internal/git/diff.go",
	)
	tree := buildTree(files)

	// "internal" has two children (ui, git) → not collapsed
	require.Len(t, tree, 1)
	assert.Equal(t, "internal", tree[0].name)
	assert.Len(t, tree[0].children, 2)
}

func TestBuildTree_DirectoriesBeforeFiles(t *testing.T) {
	files := makeFileDiffs(
		"main.go",
		"internal/ui/model.go",
	)
	tree := buildTree(files)

	require.Len(t, tree, 2)
	assert.True(t, tree[0].isDir(), "directory should come first")
	assert.False(t, tree[1].isDir(), "file should come second")
}

func TestFlattenTree_AllExpanded(t *testing.T) {
	files := makeFileDiffs(
		"internal/ui/model.go",
		"internal/ui/filelist.go",
		"main.go",
	)
	tree := buildTree(files)
	rows := flattenTree(tree)

	// internal/ui/ (depth 0), filelist.go (depth 1), model.go (depth 1), main.go (depth 0)
	require.Len(t, rows, 4)

	assert.True(t, rows[0].node.isDir())
	assert.Equal(t, 0, rows[0].depth)

	assert.Equal(t, "filelist.go", rows[1].node.name)
	assert.Equal(t, 1, rows[1].depth)

	assert.Equal(t, "model.go", rows[2].node.name)
	assert.Equal(t, 1, rows[2].depth)

	assert.Equal(t, "main.go", rows[3].node.name)
	assert.Equal(t, 0, rows[3].depth)
}

func TestFlattenTree_CollapsedDir(t *testing.T) {
	files := makeFileDiffs(
		"internal/ui/model.go",
		"internal/ui/filelist.go",
		"main.go",
	)
	tree := buildTree(files)
	tree[0].expanded = false // collapse internal/ui
	rows := flattenTree(tree)

	// Only: internal/ui/ (collapsed), main.go
	require.Len(t, rows, 2)
	assert.True(t, rows[0].node.isDir())
	assert.Equal(t, "main.go", rows[1].node.name)
}

func TestTreeDisplayName_Dir_AtRoot(t *testing.T) {
	node := &treeNode{name: "internal/ui", children: []*treeNode{}, expanded: true}
	row := treeRow{node: node, depth: 0, prefix: ""}
	assert.Equal(t, "▾ internal/ui/", treeDisplayName(row))
}

func TestTreeDisplayName_CollapsedDir(t *testing.T) {
	node := &treeNode{name: "internal/ui", children: []*treeNode{}, expanded: false}
	row := treeRow{node: node, depth: 0, prefix: ""}
	assert.Equal(t, "▸ internal/ui/", treeDisplayName(row))
}

func TestTreeDisplayName_WithBranchChars(t *testing.T) {
	// Build a real tree to get proper prefixes
	files := makeFileDiffs("dir/a.go", "dir/b.go", "root.go")
	tree := buildTree(files)
	rows := flattenTree(tree)

	// dir/ is not last sibling (root.go follows), so children get "│   " prefix
	require.Len(t, rows, 4)
	assert.Equal(t, "▾ dir/", treeDisplayName(rows[0]))
	assert.Equal(t, "│   ├── a.go", treeDisplayName(rows[1]))
	assert.Equal(t, "│   └── b.go", treeDisplayName(rows[2]))
	assert.Equal(t, "root.go", treeDisplayName(rows[3]))
}

func TestTreeDisplayName_WithBranchChars_LastDir(t *testing.T) {
	// When directory IS the last root sibling, children get "    " prefix
	files := makeFileDiffs("dir/a.go", "dir/b.go")
	tree := buildTree(files)
	rows := flattenTree(tree)

	require.Len(t, rows, 3)
	assert.Equal(t, "▾ dir/", treeDisplayName(rows[0]))
	assert.Equal(t, "    ├── a.go", treeDisplayName(rows[1]))
	assert.Equal(t, "    └── b.go", treeDisplayName(rows[2]))
}

func TestTreeDisplayName_NestedBranchChars(t *testing.T) {
	files := makeFileDiffs("a/b/c.go", "a/b/d.go", "a/e.go")
	tree := buildTree(files)
	rows := flattenTree(tree)

	// a/ is only root (isLast=true), so children get "    " prefix
	// b/ is not last child of a/ (e.go follows), so b/'s children get "    │   "
	require.Len(t, rows, 5)
	assert.Equal(t, "▾ a/", treeDisplayName(rows[0]))
	assert.Equal(t, "    ├── ▾ b/", treeDisplayName(rows[1]))
	assert.Equal(t, "    │   ├── c.go", treeDisplayName(rows[2]))
	assert.Equal(t, "    │   └── d.go", treeDisplayName(rows[3]))
	assert.Equal(t, "    └── e.go", treeDisplayName(rows[4]))
}

func TestBuildTree_FileIndices(t *testing.T) {
	files := makeFileDiffs("b.go", "a/c.go", "a/d.go")
	tree := buildTree(files)

	rows := flattenTree(tree)
	// a/ (dir), c.go (idx 1), d.go (idx 2), b.go (idx 0)
	require.Len(t, rows, 4)
	assert.Equal(t, 1, rows[1].node.fileIdx) // a/c.go
	assert.Equal(t, 2, rows[2].node.fileIdx) // a/d.go
	assert.Equal(t, 0, rows[3].node.fileIdx) // b.go
}

func TestBuildTree_Empty(t *testing.T) {
	tree := buildTree(nil)
	assert.Len(t, tree, 0)
}

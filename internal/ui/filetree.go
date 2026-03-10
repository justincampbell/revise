package ui

import (
	"path"
	"sort"
	"strings"

	"github.com/justincampbell/revise/internal/git"
)

// treeNode represents a node in the file tree.
// It is either a directory (children non-nil) or a file (fileIdx >= 0).
type treeNode struct {
	name     string // base name of the segment
	path     string // full path
	fileIdx  int    // index into []git.FileDiff, -1 for directories
	children []*treeNode
	expanded bool
}

func (n *treeNode) isDir() bool {
	return n.children != nil
}

// buildTree creates a tree from a flat list of file diffs.
// Files are grouped by directory. Single-child directories are collapsed
// (e.g. "internal/ui" instead of "internal" → "ui").
func buildTree(files []git.FileDiff) []*treeNode {
	root := &treeNode{children: []*treeNode{}}
	for i, f := range files {
		parts := strings.Split(f.Path, "/")
		insertNode(root, parts, i)
	}
	collapseChains(root)
	return root.children
}

func insertNode(parent *treeNode, parts []string, fileIdx int) {
	if len(parts) == 1 {
		parent.children = append(parent.children, &treeNode{
			name:    parts[0],
			path:    parts[0],
			fileIdx: fileIdx,
		})
		return
	}

	dirName := parts[0]
	var dir *treeNode
	for _, c := range parent.children {
		if c.isDir() && c.name == dirName {
			dir = c
			break
		}
	}
	if dir == nil {
		dir = &treeNode{
			name:     dirName,
			path:     dirName,
			fileIdx:  -1,
			children: []*treeNode{},
			expanded: true,
		}
		parent.children = append(parent.children, dir)
	}
	insertNode(dir, parts[1:], fileIdx)
}

// collapseChains merges single-child directory chains like a/b/c → "a/b/c".
func collapseChains(node *treeNode) {
	for _, child := range node.children {
		if child.isDir() {
			collapseChains(child)
			// Collapse: if this dir has exactly one child that is also a dir, merge them.
			for len(child.children) == 1 && child.children[0].isDir() {
				grandchild := child.children[0]
				child.name = child.name + "/" + grandchild.name
				child.path = child.name
				child.children = grandchild.children
			}
		}
	}
	sortChildren(node)
}

func sortChildren(node *treeNode) {
	sort.Slice(node.children, func(i, j int) bool {
		a, b := node.children[i], node.children[j]
		// Directories first, then alphabetical
		if a.isDir() != b.isDir() {
			return a.isDir()
		}
		return a.name < b.name
	})
	for _, c := range node.children {
		if c.isDir() {
			sortChildren(c)
		}
	}
}

// flattenTree produces the visible rows from a tree, respecting expanded state.
type treeRow struct {
	node  *treeNode
	depth int
}

func flattenTree(nodes []*treeNode) []treeRow {
	var rows []treeRow
	flattenNodes(nodes, 0, &rows)
	return rows
}

func flattenNodes(nodes []*treeNode, depth int, rows *[]treeRow) {
	for _, n := range nodes {
		*rows = append(*rows, treeRow{node: n, depth: depth})
		if n.isDir() && n.expanded {
			flattenNodes(n.children, depth+1, rows)
		}
	}
}

// treeDisplayName returns the display name for a tree row.
func treeDisplayName(row treeRow) string {
	indent := strings.Repeat("  ", row.depth)
	if row.node.isDir() {
		arrow := "▸"
		if row.node.expanded {
			arrow = "▾"
		}
		return indent + arrow + " " + row.node.name + "/"
	}
	return indent + "  " + row.node.name
}

// treeFileStatus returns the status indicator for a tree row, or "" for directories.
func treeFileStatus(row treeRow, files []git.FileDiff) git.FileStatus {
	if row.node.isDir() {
		return ""
	}
	if row.node.fileIdx >= 0 && row.node.fileIdx < len(files) {
		return files[row.node.fileIdx].Status
	}
	return ""
}

// filePath returns the full path for a tree node by reconstructing from parent context.
func treeFilePath(row treeRow, files []git.FileDiff) string {
	if row.node.fileIdx >= 0 && row.node.fileIdx < len(files) {
		return files[row.node.fileIdx].Path
	}
	return ""
}

// treeRootPath returns the directory path by joining ancestor names.
// For building the tree display, we just use the node name since chains are collapsed.
func treeRootPath(node *treeNode) string {
	return path.Clean(node.name)
}

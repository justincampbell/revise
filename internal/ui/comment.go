package ui

import (
	"fmt"
	"sort"
	"strings"

	revisecomments "github.com/justincampbell/revise/internal/comments"
	"github.com/justincampbell/revise/internal/git"
)

type commentKey struct {
	file    string
	lineNum int
	isOld   bool // true for removed lines (keyed by old line number)
}

// commentEntry holds a single user comment along with its persistent ID.
type commentEntry struct {
	ID   string // stable identifier used by the MCP server
	Body string // the comment text entered by the user
}

type comments map[commentKey]commentEntry

func (c comments) isEmpty() bool {
	return len(c) == 0
}

func (c comments) countForFile(path string) int {
	n := 0
	for k := range c {
		if k.file == path {
			n++
		}
	}
	return n
}

// formatExport formats all comments for export in a readable format suitable for Claude Code.
// Files are listed in the order they appear in the diff.
func formatExport(files []git.FileDiff, c comments) string {
	if len(c) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# Code Review Comments\n")

	for _, f := range files {
		type lineComment struct {
			lineNum int
			isOld   bool
			text    string
		}
		var fileComments []lineComment
		for key, entry := range c {
			if key.file == f.Path {
				fileComments = append(fileComments, lineComment{key.lineNum, key.isOld, entry.Body})
			}
		}
		if len(fileComments) == 0 {
			continue
		}
		sort.Slice(fileComments, func(i, j int) bool {
			return fileComments[i].lineNum < fileComments[j].lineNum
		})

		sb.WriteString(fmt.Sprintf("\n## %s\n\n", f.Path))
		for _, lc := range fileComments {
			if lc.isOld {
				sb.WriteString(fmt.Sprintf("Line %d (removed): %s\n", lc.lineNum, lc.text))
			} else {
				sb.WriteString(fmt.Sprintf("Line %d: %s\n", lc.lineNum, lc.text))
			}
		}
	}

	return sb.String()
}

// toReviewComments converts in-memory comments to structured ReviewFile comments,
// sorted by file path then line number for a stable output order.
func toReviewComments(c comments) []revisecomments.Comment {
	type item struct {
		key   commentKey
		entry commentEntry
	}
	items := make([]item, 0, len(c))
	for k, e := range c {
		items = append(items, item{k, e})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].key.file != items[j].key.file {
			return items[i].key.file < items[j].key.file
		}
		return items[i].key.lineNum < items[j].key.lineNum
	})

	result := make([]revisecomments.Comment, len(items))
	for i, it := range items {
		side := "new"
		if it.key.isOld {
			side = "old"
		}
		result[i] = revisecomments.Comment{
			ID:     it.entry.ID,
			File:   it.key.file,
			Line:   it.key.lineNum,
			Side:   side,
			Body:   it.entry.Body,
			Status: "open",
		}
	}
	return result
}

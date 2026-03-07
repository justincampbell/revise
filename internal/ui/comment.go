package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/justincampbell/revise/internal/git"
)

type commentKey struct {
	file    string
	lineNum int
}

type comments map[commentKey]string

func (c comments) isEmpty() bool {
	return len(c) == 0
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
			text    string
		}
		var fileComments []lineComment
		for key, text := range c {
			if key.file == f.Path {
				fileComments = append(fileComments, lineComment{key.lineNum, text})
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
			sb.WriteString(fmt.Sprintf("Line %d: %s\n", lc.lineNum, lc.text))
		}
	}

	return sb.String()
}

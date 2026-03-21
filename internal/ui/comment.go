package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/justincampbell/revise/internal/git"
)

type commentKey struct {
	file    string
	lineNum int
	isOld   bool // true for removed lines (keyed by old line number)
}

// encode returns a string representation of the key for serialization.
func (k commentKey) encode() string {
	return fmt.Sprintf("%s:%d:%t", k.file, k.lineNum, k.isOld)
}

// decodeCommentKey parses a string produced by commentKey.encode().
// Returns the zero value and false if parsing fails.
func decodeCommentKey(s string) (commentKey, bool) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return commentKey{}, false
	}
	lineNum, err := strconv.Atoi(parts[1])
	if err != nil {
		return commentKey{}, false
	}
	isOld, err := strconv.ParseBool(parts[2])
	if err != nil {
		return commentKey{}, false
	}
	return commentKey{file: parts[0], lineNum: lineNum, isOld: isOld}, true
}

type comments map[commentKey]string

// toStringMap converts comments to a serializable string-keyed map.
func (c comments) toStringMap() map[string]string {
	m := make(map[string]string, len(c))
	for k, v := range c {
		m[k.encode()] = v
	}
	return m
}

// commentsFromStringMap converts a string-keyed map back to comments.
// Invalid keys are silently skipped.
func commentsFromStringMap(m map[string]string) comments {
	c := make(comments, len(m))
	for k, v := range m {
		if key, ok := decodeCommentKey(k); ok {
			c[key] = v
		}
	}
	return c
}

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

// findLineContent looks up the content of a line in a file's hunks.
func findLineContent(f git.FileDiff, lineNum int, isOld bool) string {
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			if isOld && l.OldNum == lineNum && (l.Type == git.LineRemoved || l.Type == git.LineContext) {
				return l.Content
			}
			if !isOld && l.NewNum == lineNum && (l.Type == git.LineAdded || l.Type == git.LineContext) {
				return l.Content
			}
		}
	}
	return ""
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
		for key, text := range c {
			if key.file == f.Path {
				fileComments = append(fileComments, lineComment{key.lineNum, key.isOld, text})
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
			if lc.lineNum == 0 {
				sb.WriteString(fmt.Sprintf("> %s\n\n", lc.text))
			} else {
				content := findLineContent(f, lc.lineNum, lc.isOld)
				if lc.isOld && content != "" {
					sb.WriteString(fmt.Sprintf("%d (removed): `%s`\n", lc.lineNum, content))
				} else if lc.isOld {
					sb.WriteString(fmt.Sprintf("%d (removed):\n", lc.lineNum))
				} else if content != "" {
					sb.WriteString(fmt.Sprintf("%d: `%s`\n", lc.lineNum, content))
				} else {
					sb.WriteString(fmt.Sprintf("%d:\n", lc.lineNum))
				}
				sb.WriteString(fmt.Sprintf("> %s\n\n", lc.text))
			}
		}
	}

	return sb.String()
}

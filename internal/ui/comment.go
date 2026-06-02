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

type marks map[commentKey]bool

type comments map[commentKey]string

// markValue is the reserved value used to persist marks in the string map.
const markValue = "__marked__"

// toStringMap converts comments and marks to a serializable string-keyed map.
func toStringMap(c comments, m marks) map[string]string {
	out := make(map[string]string, len(c)+len(m))
	for k, v := range c {
		out[k.encode()] = v
	}
	for k := range m {
		out[k.encode()] = markValue
	}
	return out
}

// fromStringMap converts a string-keyed map back to comments and marks.
// Invalid keys are silently skipped.
func fromStringMap(m map[string]string) (comments, marks) {
	c := make(comments, len(m))
	mk := make(marks)
	for k, v := range m {
		key, ok := decodeCommentKey(k)
		if !ok {
			continue
		}
		if v == markValue {
			mk[key] = true
		} else {
			c[key] = v
		}
	}
	return c, mk
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

func (m marks) countForFile(path string) int {
	n := 0
	for k := range m {
		if k.file == path {
			n++
		}
	}
	return n
}

// findLineContent looks up the content of a line in a file's hunks.
// Leading whitespace is stripped to keep exported output readable.
func findLineContent(f git.FileDiff, lineNum int, isOld bool) string {
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			if isOld && l.OldNum == lineNum && (l.Type == git.LineRemoved || l.Type == git.LineContext) {
				return strings.TrimLeft(l.Content, " \t")
			}
			if !isOld && l.NewNum == lineNum && (l.Type == git.LineAdded || l.Type == git.LineContext) {
				return strings.TrimLeft(l.Content, " \t")
			}
		}
	}
	return ""
}

// formatExport formats all comments and marks for export in a readable format suitable for Claude Code.
// Files are listed in the order they appear in the diff.
func formatExport(files []git.FileDiff, c comments, m marks) string {
	if len(c) == 0 && len(m) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# Code Review Comments\n")

	for _, f := range files {
		type lineEntry struct {
			lineNum int
			isOld   bool
			text    string // empty for marks
			isMark  bool
		}
		var entries []lineEntry
		for key, text := range c {
			if key.file == f.Path {
				entries = append(entries, lineEntry{key.lineNum, key.isOld, text, false})
			}
		}
		for key := range m {
			if key.file == f.Path {
				entries = append(entries, lineEntry{key.lineNum, key.isOld, "", true})
			}
		}
		if len(entries) == 0 {
			continue
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].lineNum < entries[j].lineNum
		})

		fmt.Fprintf(&sb, "\n## %s\n\n", f.Path)
		for _, lc := range entries {
			if lc.lineNum == 0 {
				if lc.isMark {
					fmt.Fprintf(&sb, "> [flagged]\n\n")
				} else {
					fmt.Fprintf(&sb, "> %s\n\n", lc.text)
				}
			} else {
				content := findLineContent(f, lc.lineNum, lc.isOld)
				if lc.isOld && content != "" {
					fmt.Fprintf(&sb, "%d (removed): `%s`\n", lc.lineNum, content)
				} else if lc.isOld {
					fmt.Fprintf(&sb, "%d (removed):\n", lc.lineNum)
				} else if content != "" {
					fmt.Fprintf(&sb, "%d: `%s`\n", lc.lineNum, content)
				} else {
					fmt.Fprintf(&sb, "%d:\n", lc.lineNum)
				}
				if lc.isMark {
					fmt.Fprintf(&sb, "> [flagged]\n\n")
				} else {
					fmt.Fprintf(&sb, "> %s\n\n", lc.text)
				}
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

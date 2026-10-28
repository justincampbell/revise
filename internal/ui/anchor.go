package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/justincampbell/revise/internal/git"
)

// Persisted comments are keyed by line number, so each one also stores the
// content of its line. On load, comments follow their content if the file
// changed, and become outdated if the content is gone.

// Prefixed keys fail decodeCommentKey, so older builds skip them.
const (
	anchorPrefix   = "anchor:"
	outdatedPrefix = "outdated:"
	outdatedSep    = "\x1f"
)

type outdatedComment struct {
	key  commentKey
	was  string
	text string
}

// lineSource returns line number → trimmed content for one side of a file;
// ok is false when the file no longer exists.
type lineSource func(file string, isOld bool) (lines map[int]string, ok bool)

func encodeAnchors(out map[string]string, anchors map[commentKey]string, outdated []outdatedComment) {
	for k, v := range anchors {
		out[anchorPrefix+k.encode()] = v
	}
	for _, o := range outdated {
		out[outdatedPrefix+o.key.encode()] = o.was + outdatedSep + o.text
	}
}

func decodeAnchors(m map[string]string) (map[commentKey]string, []outdatedComment) {
	anchors := make(map[commentKey]string)
	var outdated []outdatedComment
	for k, v := range m {
		if rest, ok := strings.CutPrefix(k, anchorPrefix); ok {
			if key, ok := decodeCommentKey(rest); ok {
				anchors[key] = v
			}
		} else if rest, ok := strings.CutPrefix(k, outdatedPrefix); ok {
			key, ok := decodeCommentKey(rest)
			was, text, found := strings.Cut(v, outdatedSep)
			if ok && found {
				outdated = append(outdated, outdatedComment{key: key, was: was, text: text})
			}
		}
	}
	sortOutdated(outdated)
	return anchors, outdated
}

// captureAnchors records line content for newly added comments and marks,
// and forgets anchors whose comment or mark is gone.
func captureAnchors(files []git.FileDiff, c comments, mk marks, anchors map[commentKey]string) {
	for k := range anchors {
		_, hasComment := c[k]
		if !hasComment && !mk[k] {
			delete(anchors, k)
		}
	}
	record := func(k commentKey) {
		if k.lineNum == 0 || anchors[k] != "" {
			return
		}
		for _, f := range files {
			if f.Path == k.file {
				if content := strings.TrimSpace(findLineContent(f, k.lineNum, k.isOld)); content != "" {
					anchors[k] = content
				}
				return
			}
		}
	}
	for k := range c {
		record(k)
	}
	for k := range mk {
		record(k)
	}
}

// reanchor moves anchored comments and marks to where their line content is
// now. Comments whose content is gone are removed and returned as outdated;
// marks in that state are dropped.
func reanchor(c comments, mk marks, anchors map[commentKey]string, src lineSource) []outdatedComment {
	keys := make([]commentKey, 0, len(anchors))
	for k := range anchors {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return lessKey(keys[i], keys[j]) })

	occupied := func(k commentKey) bool {
		_, ok := c[k]
		return ok || mk[k]
	}

	var outdated []outdatedComment
	cache := map[commentKey]map[int]string{}
	for _, k := range keys {
		text, isComment := c[k]
		if !isComment && !mk[k] {
			continue
		}
		want := anchors[k]
		side := commentKey{file: k.file, isOld: k.isOld}
		lines, cached := cache[side]
		if !cached {
			lines, _ = src(k.file, k.isOld)
			cache[side] = lines
		}
		if lines[k.lineNum] == want {
			continue
		}

		delete(anchors, k)
		delete(c, k)
		delete(mk, k)
		if n, ok := nearestMatch(lines, want, k, occupied); ok {
			moved := commentKey{file: k.file, lineNum: n, isOld: k.isOld}
			anchors[moved] = want
			if isComment {
				c[moved] = text
			} else {
				mk[moved] = true
			}
		} else if isComment {
			outdated = append(outdated, outdatedComment{key: k, was: want, text: text})
		}
	}
	return outdated
}

func nearestMatch(lines map[int]string, want string, from commentKey, occupied func(commentKey) bool) (int, bool) {
	best, bestDist := 0, -1
	for n, content := range lines {
		if content != want || occupied(commentKey{file: from.file, lineNum: n, isOld: from.isOld}) {
			continue
		}
		dist := max(n-from.lineNum, from.lineNum-n)
		if bestDist < 0 || dist < bestDist || (dist == bestDist && n < best) {
			best, bestDist = n, dist
		}
	}
	return best, bestDist >= 0
}

// workingTreeSource reads the new side from the working tree under root,
// falling back to the diff when the file can't be read, and the old side
// from the diff's removed and context lines.
func workingTreeSource(root string, files []git.FileDiff) lineSource {
	return func(file string, isOld bool) (map[int]string, bool) {
		if !isOld {
			if data, err := os.ReadFile(filepath.Join(root, file)); err == nil {
				out := map[int]string{}
				for i, l := range strings.Split(string(data), "\n") {
					out[i+1] = strings.TrimSpace(l)
				}
				return out, true
			}
		}
		for _, f := range files {
			if f.Path != file {
				continue
			}
			out := map[int]string{}
			for _, h := range f.Hunks {
				for _, l := range h.Lines {
					switch {
					case isOld && l.Type != git.LineAdded && l.OldNum > 0:
						out[l.OldNum] = strings.TrimSpace(l.Content)
					case !isOld && l.Type != git.LineRemoved && l.NewNum > 0:
						out[l.NewNum] = strings.TrimSpace(l.Content)
					}
				}
			}
			return out, true
		}
		return nil, false
	}
}

func lessKey(a, b commentKey) bool {
	if a.file != b.file {
		return a.file < b.file
	}
	if a.lineNum != b.lineNum {
		return a.lineNum < b.lineNum
	}
	return !a.isOld && b.isOld
}

func sortOutdated(o []outdatedComment) {
	sort.Slice(o, func(i, j int) bool { return lessKey(o[i].key, o[j].key) })
}

func formatOutdated(outdated []outdatedComment) string {
	if len(outdated) == 0 {
		return ""
	}
	sorted := append([]outdatedComment(nil), outdated...)
	sortOutdated(sorted)

	var sb strings.Builder
	sb.WriteString("# Outdated Comments\n\nThese were left in an earlier session on lines that have since changed.\n")
	file := ""
	for _, o := range sorted {
		if o.key.file != file {
			file = o.key.file
			fmt.Fprintf(&sb, "\n## %s\n", file)
		}
		label := "outdated"
		if o.key.isOld {
			label = "removed, outdated"
		}
		fmt.Fprintf(&sb, "\n%d (%s): `%s`\n> %s\n", o.key.lineNum, label, o.was, o.text)
	}
	return strings.TrimRight(sb.String(), "\n")
}

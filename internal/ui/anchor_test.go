package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justincampbell/revise/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeSource(newSide map[string][]string, oldSide map[string]map[int]string) lineSource {
	return func(file string, isOld bool) (map[int]string, bool) {
		if isOld {
			l, ok := oldSide[file]
			return l, ok
		}
		lines, ok := newSide[file]
		if !ok {
			return nil, false
		}
		out := make(map[int]string, len(lines))
		for i, l := range lines {
			out[i+1] = strings.TrimSpace(l)
		}
		return out, true
	}
}

func TestReanchor_SameContentKept(t *testing.T) {
	k := commentKey{file: "a.rb", lineNum: 2}
	c := comments{k: "rename"}
	anchors := map[commentKey]string{k: "Retries,"}
	src := fakeSource(map[string][]string{"a.rb": {"x", "  Retries,"}}, nil)

	out := reanchor(c, marks{}, anchors, src)
	assert.Empty(t, out)
	assert.Equal(t, "rename", c[k])
}

func TestReanchor_MovedContentFollows(t *testing.T) {
	k := commentKey{file: "a.rb", lineNum: 2}
	c := comments{k: "rename"}
	anchors := map[commentKey]string{k: "Retries,"}
	src := fakeSource(map[string][]string{"a.rb": {"x", "Failures,", "y", "Retries,"}}, nil)

	out := reanchor(c, marks{}, anchors, src)
	assert.Empty(t, out)
	moved := commentKey{file: "a.rb", lineNum: 4}
	assert.Equal(t, comments{moved: "rename"}, c)
	assert.Equal(t, map[commentKey]string{moved: "Retries,"}, anchors)
}

func TestReanchor_NearestFreeMatchWins(t *testing.T) {
	k := commentKey{file: "a.rb", lineNum: 5}
	other := commentKey{file: "a.rb", lineNum: 4}
	c := comments{k: "this one", other: "occupied"}
	anchors := map[commentKey]string{k: "end", other: "end"}
	lines := []string{"end", "x", "x", "end", "x", "x", "x", "end"}
	src := fakeSource(map[string][]string{"a.rb": lines}, nil)

	out := reanchor(c, marks{}, anchors, src)
	assert.Empty(t, out)
	assert.Equal(t, "occupied", c[other], "line 4 still matches, so it stays")
	assert.Equal(t, "this one", c[commentKey{file: "a.rb", lineNum: 8}], "8 is 3 away, 1 is 4 away")
}

func TestReanchor_GoneContentBecomesOutdated(t *testing.T) {
	k := commentKey{file: "a.rb", lineNum: 2}
	c := comments{k: "rename"}
	anchors := map[commentKey]string{k: "Retries,"}
	src := fakeSource(map[string][]string{"a.rb": {"x", "Failures,"}}, nil)

	out := reanchor(c, marks{}, anchors, src)
	assert.Equal(t, []outdatedComment{{key: k, was: "Retries,", text: "rename"}}, out)
	assert.Empty(t, c)
	assert.Empty(t, anchors)
}

func TestReanchor_MissingFileBecomesOutdated(t *testing.T) {
	k := commentKey{file: "gone.rb", lineNum: 1}
	c := comments{k: "why"}
	out := reanchor(c, marks{}, map[commentKey]string{k: "def x"}, fakeSource(nil, nil))
	require.Len(t, out, 1)
	assert.Empty(t, c)
}

func TestReanchor_GoneMarkIsDropped(t *testing.T) {
	k := commentKey{file: "a.rb", lineNum: 2}
	mk := marks{k: true}
	out := reanchor(comments{}, mk, map[commentKey]string{k: "old"}, fakeSource(map[string][]string{"a.rb": {"x", "new"}}, nil))
	assert.Empty(t, out)
	assert.Empty(t, mk)
}

func TestReanchor_MovedMarkFollows(t *testing.T) {
	k := commentKey{file: "a.rb", lineNum: 1}
	mk := marks{k: true}
	reanchor(comments{}, mk, map[commentKey]string{k: "old"}, fakeSource(map[string][]string{"a.rb": {"x", "old"}}, nil))
	assert.Equal(t, marks{commentKey{file: "a.rb", lineNum: 2}: true}, mk)
}

func TestReanchor_UnanchoredAndFileLevelKept(t *testing.T) {
	legacy := commentKey{file: "a.rb", lineNum: 2}
	fileLevel := commentKey{file: "a.rb", lineNum: 0}
	c := comments{legacy: "legacy", fileLevel: "whole file"}
	out := reanchor(c, marks{}, map[commentKey]string{}, fakeSource(map[string][]string{"a.rb": {"x", "y"}}, nil))
	assert.Empty(t, out)
	assert.Len(t, c, 2)
}

func TestReanchor_OldSideUsesOldLines(t *testing.T) {
	k := commentKey{file: "a.rb", lineNum: 3, isOld: true}
	c := comments{k: "why remove"}
	anchors := map[commentKey]string{k: "legacy_call"}
	src := fakeSource(map[string][]string{"a.rb": {"a", "b", "c"}}, map[string]map[int]string{"a.rb": {7: "legacy_call"}})

	out := reanchor(c, marks{}, anchors, src)
	assert.Empty(t, out)
	assert.Equal(t, "why remove", c[commentKey{file: "a.rb", lineNum: 7, isOld: true}])
}

func TestCaptureAnchors_RecordsAndPrunes(t *testing.T) {
	files := []git.FileDiff{{
		Path: "a.rb",
		Hunks: []git.Hunk{{Lines: []git.Line{
			{Type: git.LineAdded, Content: "  Retries,", NewNum: 26},
			{Type: git.LineRemoved, Content: "OldThing", OldNum: 30},
		}}},
	}}
	commented := commentKey{file: "a.rb", lineNum: 26}
	marked := commentKey{file: "a.rb", lineNum: 30, isOld: true}
	deleted := commentKey{file: "a.rb", lineNum: 1}
	anchors := map[commentKey]string{deleted: "stale"}

	captureAnchors(files, comments{commented: "x", {file: "a.rb"}: "file-level"}, marks{marked: true}, anchors)
	assert.Equal(t, map[commentKey]string{commented: "Retries,", marked: "OldThing"}, anchors)
}

func TestCaptureAnchors_KeepsExistingAnchor(t *testing.T) {
	k := commentKey{file: "a.rb", lineNum: 26}
	anchors := map[commentKey]string{k: "original"}
	captureAnchors(nil, comments{k: "x"}, marks{}, anchors)
	assert.Equal(t, "original", anchors[k])
}

func TestAnchorsRoundTrip(t *testing.T) {
	k := commentKey{file: "lib/a.rb", lineNum: 26}
	anchors := map[commentKey]string{k: "Retries,"}
	outdated := []outdatedComment{{key: commentKey{file: "lib/b.rb", lineNum: 4, isOld: true}, was: "old line", text: "why"}}

	m := toStringMap(comments{k: "rename"}, marks{})
	encodeAnchors(m, anchors, outdated)

	c, mk := fromStringMap(m)
	assert.Equal(t, comments{k: "rename"}, c, "prefixed keys are invisible to the plain decoder")
	assert.Empty(t, mk)

	gotAnchors, gotOutdated := decodeAnchors(m)
	assert.Equal(t, anchors, gotAnchors)
	assert.Equal(t, outdated, gotOutdated)
}

func TestWorkingTreeSource(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.rb"), []byte("one\n  two\n"), 0o644))
	files := []git.FileDiff{
		{Path: "a.rb", Hunks: []git.Hunk{{Lines: []git.Line{{Type: git.LineRemoved, Content: "old", OldNum: 5}}}}},
		{Path: "gone.rb", Hunks: []git.Hunk{{Lines: []git.Line{{Type: git.LineRemoved, Content: "x", OldNum: 1}}}}},
	}
	src := workingTreeSource(root, files)

	lines, ok := src("a.rb", false)
	require.True(t, ok)
	assert.Equal(t, "two", lines[2])

	lines, ok = src("a.rb", true)
	require.True(t, ok)
	assert.Equal(t, map[int]string{5: "old"}, lines)

	lines, ok = src("gone.rb", false)
	assert.True(t, ok, "falls back to the diff")
	assert.Empty(t, lines, "a deleted file has no new-side lines")

	_, ok = src("never.rb", false)
	assert.False(t, ok)
}

// Reproduces a comment restored onto whatever line now sits at its old number
// after the branch was rewritten between runs.
func TestStore_RewrittenBranchReanchorsAndOutdates(t *testing.T) {
	before := []git.Line{
		{Type: git.LineAdded, Content: "  Retries,", NewNum: 26},
		{Type: git.LineAdded, Content: "  Timeouts,", NewNum: 27},
	}
	m, storePath := makeModelWithStore(t, "lib/metrics.rb", before)
	m.comments[commentKey{file: "lib/metrics.rb", lineNum: 26}] = "per-host?"
	m.comments[commentKey{file: "lib/metrics.rb", lineNum: 27}] = "rename"
	m.saveComments()

	after := []git.Line{
		{Type: git.LineAdded, Content: "  Failures,", NewNum: 26},
		{Type: git.LineAdded, Content: "  Retries,", NewNum: 28},
	}
	m2 := NewWithStorePath(&git.Diff{Files: []git.FileDiff{{
		Path: "lib/metrics.rb", Status: git.StatusModified,
		Hunks: []git.Hunk{{Lines: after}},
	}}}, false, storePath, "")

	assert.Equal(t, comments{{file: "lib/metrics.rb", lineNum: 28}: "per-host?"}, m2.comments)
	require.Len(t, m2.outdated, 1)
	assert.Equal(t, "Timeouts,", m2.outdated[0].was)

	out := m2.ExportedComments()
	assert.Contains(t, out, "28: `Retries,`\n> per-host?")
	assert.Contains(t, out, "# Outdated Comments")
	assert.Contains(t, out, "27 (outdated): `Timeouts,`\n> rename")
	assert.NotContains(t, out, "26:")
}

func TestFormatOutdated(t *testing.T) {
	assert.Equal(t, "", formatOutdated(nil))
	out := formatOutdated([]outdatedComment{
		{key: commentKey{file: "b.rb", lineNum: 9}, was: "y", text: "second"},
		{key: commentKey{file: "a.rb", lineNum: 26}, was: "Retries,", text: "rename"},
		{key: commentKey{file: "a.rb", lineNum: 3, isOld: true}, was: "gone()", text: "why"},
	})
	assert.True(t, strings.HasPrefix(out, "# Outdated Comments\n"), out)
	assert.Contains(t, out, "## a.rb\n\n3 (removed, outdated): `gone()`\n> why\n\n26 (outdated): `Retries,`\n> rename")
	assert.Less(t, strings.Index(out, "## a.rb"), strings.Index(out, "## b.rb"))
}

func TestExportedComments_IncludesOutdatedAfterCurrent(t *testing.T) {
	m := makeModel("a.go")
	m.comments[commentKey{file: "a.go", lineNum: 3}] = "current"
	m.outdated = []outdatedComment{{key: commentKey{file: "a.go", lineNum: 26}, was: "old", text: "stale"}}
	out := m.ExportedComments()
	assert.Less(t, strings.Index(out, "# Code Review Comments"), strings.Index(out, "# Outdated Comments"))

	m.comments = comments{}
	assert.True(t, strings.HasPrefix(m.ExportedComments(), "# Outdated Comments"))
}

func TestClearAll_ClearsOutdated(t *testing.T) {
	m := makeModel("a.go")
	m.outdated = []outdatedComment{{key: commentKey{file: "a.go", lineNum: 26}, was: "old", text: "stale"}}
	m = sendKey(m, "C")
	m = sendKey(m, "y")
	assert.Empty(t, m.outdated)
	assert.Equal(t, "", m.ExportedComments())
}

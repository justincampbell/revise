package ui

import (
	"strings"
	"testing"

	"github.com/justincampbell/revise/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatExport_Empty(t *testing.T) {
	result := formatExport(nil, comments{}, nil)
	assert.Equal(t, "", result)
}

func TestFormatExport_SingleComment(t *testing.T) {
	files := []git.FileDiff{{
		Path: "foo.go",
		Hunks: []git.Hunk{{
			Lines: []git.Line{
				{Type: git.LineAdded, Content: "hello world", NewNum: 10},
			},
		}},
	}}
	c := comments{
		{file: "foo.go", lineNum: 10}: "fix this",
	}
	result := formatExport(files, c, nil)
	assert.Contains(t, result, "## foo.go")
	assert.Contains(t, result, "10: `hello world`\n> fix this")
}

func TestFormatExport_StripsLeadingWhitespace(t *testing.T) {
	files := []git.FileDiff{{
		Path: "comment_test.go",
		Hunks: []git.Hunk{{
			Lines: []git.Line{
				{Type: git.LineAdded, Content: "\t\tHunks: []git.Hunk{{", NewNum: 211},
			},
		}},
	}}
	c := comments{
		{file: "comment_test.go", lineNum: 211}: "Test",
	}
	result := formatExport(files, c)
	assert.Contains(t, result, "211: `Hunks: []git.Hunk{{`\n> Test")
	assert.NotContains(t, result, "\t\tHunks")
}

func TestFormatExport_MultipleFiles_OrderedByDiff(t *testing.T) {
	files := []git.FileDiff{
		{Path: "a.go"},
		{Path: "b.go"},
	}
	c := comments{
		{file: "b.go", lineNum: 5}: "check b",
		{file: "a.go", lineNum: 3}: "check a",
	}
	result := formatExport(files, c, nil)
	aIdx := strings.Index(result, "## a.go")
	bIdx := strings.Index(result, "## b.go")
	assert.Greater(t, aIdx, -1)
	assert.Greater(t, bIdx, -1)
	assert.Less(t, aIdx, bIdx)
}

func TestFormatExport_LinesOrderedAscending(t *testing.T) {
	files := []git.FileDiff{{Path: "foo.go"}}
	c := comments{
		{file: "foo.go", lineNum: 20}: "later",
		{file: "foo.go", lineNum: 5}:  "earlier",
	}
	result := formatExport(files, c, nil)
	earlierIdx := strings.Index(result, "5:")
	laterIdx := strings.Index(result, "20:")
	assert.Less(t, earlierIdx, laterIdx)
}

func TestCountForFile(t *testing.T) {
	c := comments{
		{file: "a.go", lineNum: 1}: "one",
		{file: "a.go", lineNum: 2}: "two",
		{file: "b.go", lineNum: 1}: "other",
	}
	assert.Equal(t, 2, c.countForFile("a.go"))
	assert.Equal(t, 1, c.countForFile("b.go"))
	assert.Equal(t, 0, c.countForFile("c.go"))
}

func TestFormatExport_ExcludesFilesWithNoComments(t *testing.T) {
	files := []git.FileDiff{
		{Path: "a.go"},
		{Path: "b.go"},
	}
	c := comments{
		{file: "a.go", lineNum: 1}: "only a",
	}
	result := formatExport(files, c, nil)
	assert.Contains(t, result, "## a.go")
	assert.NotContains(t, result, "## b.go")
}

func TestFormatExport_RemovedLineIncludesContent(t *testing.T) {
	files := []git.FileDiff{{
		Path: "foo.go",
		Hunks: []git.Hunk{{
			Lines: []git.Line{
				{Type: git.LineRemoved, Content: "old code", OldNum: 5},
			},
		}},
	}}
	c := comments{
		{file: "foo.go", lineNum: 5, isOld: true}: "why was this removed?",
	}
	result := formatExport(files, c, nil)
	assert.Contains(t, result, "5 (removed): `old code`\n> why was this removed?")
}

func TestFormatExport_LineContentNotFound(t *testing.T) {
	files := []git.FileDiff{{Path: "foo.go"}}
	c := comments{
		{file: "foo.go", lineNum: 99}: "orphaned comment",
	}
	result := formatExport(files, c, nil)
	assert.Contains(t, result, "99:\n> orphaned comment")
}

// --- commentKey encode/decode tests ---

func TestCommentKey_EncodeRoundTrip(t *testing.T) {
	key := commentKey{file: "foo.go", lineNum: 42, isOld: false}
	encoded := key.encode()
	decoded, ok := decodeCommentKey(encoded)
	require.True(t, ok)
	assert.Equal(t, key, decoded)
}

func TestCommentKey_EncodeRoundTrip_OldLine(t *testing.T) {
	key := commentKey{file: "bar.go", lineNum: 7, isOld: true}
	encoded := key.encode()
	decoded, ok := decodeCommentKey(encoded)
	require.True(t, ok)
	assert.Equal(t, key, decoded)
}

func TestDecodeCommentKey_InvalidFormat(t *testing.T) {
	_, ok := decodeCommentKey("invalid")
	assert.False(t, ok)
}

func TestDecodeCommentKey_InvalidLineNum(t *testing.T) {
	_, ok := decodeCommentKey("file.go:abc:false")
	assert.False(t, ok)
}

func TestDecodeCommentKey_InvalidBool(t *testing.T) {
	_, ok := decodeCommentKey("file.go:1:notbool")
	assert.False(t, ok)
}

func TestToStringMap_RoundTrip(t *testing.T) {
	c := make(comments)
	c[commentKey{file: "a.go", lineNum: 1, isOld: false}] = "fix"
	c[commentKey{file: "b.go", lineNum: 5, isOld: true}] = "why?"

	mk := marks{
		{file: "c.go", lineNum: 3}: true,
	}

	m := toStringMap(c, mk)
	assert.Len(t, m, 3)

	restoredC, restoredM := fromStringMap(m)
	assert.Equal(t, c, restoredC)
	assert.Equal(t, mk, restoredM)
}

func TestFromStringMap_SkipsInvalidKeys(t *testing.T) {
	m := map[string]string{
		"valid.go:1:false": "ok",
		"invalid":          "skip",
	}
	c, mk := fromStringMap(m)
	assert.Len(t, c, 1)
	assert.Len(t, mk, 0)
	assert.Equal(t, "ok", c[commentKey{file: "valid.go", lineNum: 1, isOld: false}])
}

func TestFromStringMap_SeparatesMarks(t *testing.T) {
	m := map[string]string{
		"a.go:1:false": "comment",
		"b.go:2:false": markValue,
	}
	c, mk := fromStringMap(m)
	assert.Len(t, c, 1)
	assert.Equal(t, "comment", c[commentKey{file: "a.go", lineNum: 1}])
	assert.Len(t, mk, 1)
	assert.True(t, mk[commentKey{file: "b.go", lineNum: 2}])
}

func TestCommentKey_FileLevelComment_EncodeRoundTrip(t *testing.T) {
	key := commentKey{file: "foo.go", lineNum: 0, isOld: false}
	encoded := key.encode()
	decoded, ok := decodeCommentKey(encoded)
	require.True(t, ok)
	assert.Equal(t, key, decoded)
}

func TestFormatExport_FileLevelComment(t *testing.T) {
	files := []git.FileDiff{{
		Path: "foo.go",
		Hunks: []git.Hunk{{
			Lines: []git.Line{
				{Type: git.LineAdded, Content: "some code", NewNum: 10},
			},
		}},
	}}
	c := comments{
		{file: "foo.go", lineNum: 0}:  "file comment",
		{file: "foo.go", lineNum: 10}: "line comment",
	}
	result := formatExport(files, c, nil)
	assert.Contains(t, result, "## foo.go")
	assert.Contains(t, result, "> file comment")
	assert.Contains(t, result, "10: `some code`\n> line comment")
}

func TestFormatExport_FileLevelComment_AppearsFirst(t *testing.T) {
	files := []git.FileDiff{{Path: "foo.go"}}
	c := comments{
		{file: "foo.go", lineNum: 0}:  "file comment",
		{file: "foo.go", lineNum: 10}: "line comment",
	}
	result := formatExport(files, c, nil)
	fileIdx := strings.Index(result, "> file comment")
	lineIdx := strings.Index(result, "10:")
	assert.Greater(t, fileIdx, -1)
	assert.Greater(t, lineIdx, -1)
	assert.Less(t, fileIdx, lineIdx, "file-level comment should appear before line comments")
}

func TestFormatExport_NoTrailingBlankLines(t *testing.T) {
	files := []git.FileDiff{{
		Path: "foo.go",
		Hunks: []git.Hunk{{
			Lines: []git.Line{
				{Type: git.LineAdded, Content: "hello", NewNum: 10},
			},
		}},
	}}
	c := comments{
		{file: "foo.go", lineNum: 10}: "fix this",
	}
	result := formatExport(files, c, nil)
	assert.NotEqual(t, "", result)
	assert.Equal(t, strings.TrimRight(result, "\n"), result, "export should not end with blank lines")
}

func TestFormatExport_Mark(t *testing.T) {
	files := []git.FileDiff{{
		Path: "foo.go",
		Hunks: []git.Hunk{{
			Lines: []git.Line{
				{Type: git.LineAdded, Content: "flagged line", NewNum: 7},
			},
		}},
	}}
	m := marks{
		{file: "foo.go", lineNum: 7}: true,
	}
	result := formatExport(files, comments{}, m)
	assert.Contains(t, result, "7: `flagged line`")
	assert.Contains(t, result, "> [flagged]")
}

func TestCountForFile_IncludesFileLevel(t *testing.T) {
	c := comments{
		{file: "a.go", lineNum: 0}: "file comment",
		{file: "a.go", lineNum: 1}: "line comment",
	}
	assert.Equal(t, 2, c.countForFile("a.go"))
}

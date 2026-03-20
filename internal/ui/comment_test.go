package ui

import (
	"strings"
	"testing"

	"github.com/justincampbell/revise/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatExport_Empty(t *testing.T) {
	result := formatExport(nil, comments{})
	assert.Equal(t, "", result)
}

func TestFormatExport_SingleComment(t *testing.T) {
	files := []git.FileDiff{{Path: "foo.go"}}
	c := comments{
		{file: "foo.go", lineNum: 10}: "fix this",
	}
	result := formatExport(files, c)
	assert.Contains(t, result, "## foo.go")
	assert.Contains(t, result, "Line 10: fix this")
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
	result := formatExport(files, c)
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
	result := formatExport(files, c)
	earlierIdx := strings.Index(result, "Line 5:")
	laterIdx := strings.Index(result, "Line 20:")
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
	result := formatExport(files, c)
	assert.Contains(t, result, "## a.go")
	assert.NotContains(t, result, "## b.go")
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

func TestComments_ToStringMap_RoundTrip(t *testing.T) {
	c := make(comments)
	c[commentKey{file: "a.go", lineNum: 1, isOld: false}] = "fix"
	c[commentKey{file: "b.go", lineNum: 5, isOld: true}] = "why?"

	m := c.toStringMap()
	assert.Len(t, m, 2)

	restored := commentsFromStringMap(m)
	assert.Equal(t, c, restored)
}

func TestCommentsFromStringMap_SkipsInvalidKeys(t *testing.T) {
	m := map[string]string{
		"valid.go:1:false": "ok",
		"invalid":          "skip",
	}
	c := commentsFromStringMap(m)
	assert.Len(t, c, 1)
	assert.Equal(t, "ok", c[commentKey{file: "valid.go", lineNum: 1, isOld: false}])
}

func TestCommentKey_FileLevelComment_EncodeRoundTrip(t *testing.T) {
	key := commentKey{file: "foo.go", lineNum: 0, isOld: false}
	encoded := key.encode()
	decoded, ok := decodeCommentKey(encoded)
	require.True(t, ok)
	assert.Equal(t, key, decoded)
}

func TestFormatExport_FileLevelComment(t *testing.T) {
	files := []git.FileDiff{{Path: "foo.go"}}
	c := comments{
		{file: "foo.go", lineNum: 0}:  "file comment",
		{file: "foo.go", lineNum: 10}: "line comment",
	}
	result := formatExport(files, c)
	assert.Contains(t, result, "## foo.go")
	assert.Contains(t, result, "File comment: file comment")
	assert.Contains(t, result, "Line 10: line comment")
}

func TestFormatExport_FileLevelComment_AppearsFirst(t *testing.T) {
	files := []git.FileDiff{{Path: "foo.go"}}
	c := comments{
		{file: "foo.go", lineNum: 0}:  "file comment",
		{file: "foo.go", lineNum: 10}: "line comment",
	}
	result := formatExport(files, c)
	fileIdx := strings.Index(result, "File comment:")
	lineIdx := strings.Index(result, "Line 10:")
	assert.Greater(t, fileIdx, -1)
	assert.Greater(t, lineIdx, -1)
	assert.Less(t, fileIdx, lineIdx, "file-level comment should appear before line comments")
}

func TestCountForFile_IncludesFileLevel(t *testing.T) {
	c := comments{
		{file: "a.go", lineNum: 0}: "file comment",
		{file: "a.go", lineNum: 1}: "line comment",
	}
	assert.Equal(t, 2, c.countForFile("a.go"))
}

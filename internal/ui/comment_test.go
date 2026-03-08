package ui

import (
	"strings"
	"testing"

	"github.com/justincampbell/revise/internal/git"
	"github.com/stretchr/testify/assert"
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

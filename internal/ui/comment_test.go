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
		{file: "foo.go", lineNum: 10}: {ID: "a1b2c3", Body: "fix this"},
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
		{file: "b.go", lineNum: 5}: {ID: "x1", Body: "check b"},
		{file: "a.go", lineNum: 3}: {ID: "x2", Body: "check a"},
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
		{file: "foo.go", lineNum: 20}: {ID: "y1", Body: "later"},
		{file: "foo.go", lineNum: 5}:  {ID: "y2", Body: "earlier"},
	}
	result := formatExport(files, c)
	earlierIdx := strings.Index(result, "Line 5:")
	laterIdx := strings.Index(result, "Line 20:")
	assert.Less(t, earlierIdx, laterIdx)
}

func TestCountForFile(t *testing.T) {
	c := comments{
		{file: "a.go", lineNum: 1}: {ID: "z1", Body: "one"},
		{file: "a.go", lineNum: 2}: {ID: "z2", Body: "two"},
		{file: "b.go", lineNum: 1}: {ID: "z3", Body: "other"},
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
		{file: "a.go", lineNum: 1}: {ID: "w1", Body: "only a"},
	}
	result := formatExport(files, c)
	assert.Contains(t, result, "## a.go")
	assert.NotContains(t, result, "## b.go")
}

func TestToReviewComments_Basic(t *testing.T) {
	c := comments{
		{file: "foo.go", lineNum: 10, isOld: false}: {ID: "abc", Body: "new line comment"},
		{file: "foo.go", lineNum: 5, isOld: true}:   {ID: "def", Body: "old line comment"},
	}
	result := toReviewComments(c)
	assert.Len(t, result, 2)

	// Find each comment
	var newComment, oldComment *struct{ side, id, body, file string }
	for _, r := range result {
		if r.ID == "abc" {
			newComment = &struct{ side, id, body, file string }{r.Side, r.ID, r.Body, r.File}
		}
		if r.ID == "def" {
			oldComment = &struct{ side, id, body, file string }{r.Side, r.ID, r.Body, r.File}
		}
	}

	assert.NotNil(t, newComment)
	assert.Equal(t, "new", newComment.side)
	assert.Equal(t, "foo.go", newComment.file)

	assert.NotNil(t, oldComment)
	assert.Equal(t, "old", oldComment.side)
}

func TestToReviewComments_Sorted(t *testing.T) {
	c := comments{
		{file: "b.go", lineNum: 1}: {ID: "b1", Body: "b file"},
		{file: "a.go", lineNum: 5}: {ID: "a5", Body: "a later"},
		{file: "a.go", lineNum: 2}: {ID: "a2", Body: "a earlier"},
	}
	result := toReviewComments(c)
	assert.Len(t, result, 3)
	// Should be sorted: a.go line 2, a.go line 5, b.go line 1
	assert.Equal(t, "a.go", result[0].File)
	assert.Equal(t, 2, result[0].Line)
	assert.Equal(t, "a.go", result[1].File)
	assert.Equal(t, 5, result[1].Line)
	assert.Equal(t, "b.go", result[2].File)
}

func TestToReviewComments_StatusOpen(t *testing.T) {
	c := comments{
		{file: "foo.go", lineNum: 1}: {ID: "x1", Body: "hello"},
	}
	result := toReviewComments(c)
	assert.Equal(t, "open", result[0].Status)
}

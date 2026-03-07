package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeFileDiffs_ReplacesSamePath(t *testing.T) {
	branch := []FileDiff{
		{Path: "a.go", Status: StatusModified},
		{Path: "b.go", Status: StatusModified},
	}
	wt := []FileDiff{
		{Path: "a.go", Status: StatusAdded},
	}

	result := mergeFileDiffs(branch, wt)

	require.Len(t, result, 2)
	assert.Equal(t, StatusAdded, result[0].Status, "a.go should be replaced by wt entry")
	assert.Equal(t, "b.go", result[1].Path, "b.go should be unchanged")
}

func TestMergeFileDiffs_AppendsNewPath(t *testing.T) {
	branch := []FileDiff{
		{Path: "a.go", Status: StatusModified},
	}
	wt := []FileDiff{
		{Path: "new.go", Status: StatusUntracked},
	}

	result := mergeFileDiffs(branch, wt)

	require.Len(t, result, 2)
	assert.Equal(t, "new.go", result[1].Path)
}

func TestMergeFileDiffs_EmptyInputs(t *testing.T) {
	result := mergeFileDiffs(nil, nil)
	assert.Empty(t, result)
}

func TestMergeFileDiffs_EmptyWT(t *testing.T) {
	branch := []FileDiff{
		{Path: "a.go", Status: StatusModified},
	}
	result := mergeFileDiffs(branch, nil)
	require.Len(t, result, 1)
	assert.Equal(t, "a.go", result[0].Path)
}

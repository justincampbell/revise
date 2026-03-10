package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeFileDiffs_CombinesHunksForSamePath(t *testing.T) {
	branch := []FileDiff{
		{Path: "a.go", Status: StatusModified, Hunks: []Hunk{{Header: "@@ -1 +1 @@", Source: SourceStaged}}},
		{Path: "b.go", Status: StatusModified},
	}
	wt := []FileDiff{
		{Path: "a.go", Status: StatusModified, Hunks: []Hunk{{Header: "@@ -2 +2 @@", Source: SourceUnstaged}}},
	}

	result := mergeFileDiffs(branch, wt)

	require.Len(t, result, 2)
	assert.Equal(t, "a.go", result[0].Path)
	require.Len(t, result[0].Hunks, 2, "a.go should have hunks from both")
	assert.Equal(t, SourceStaged, result[0].Hunks[0].Source)
	assert.Equal(t, SourceUnstaged, result[0].Hunks[1].Source)
	assert.Equal(t, "b.go", result[1].Path)
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

func TestMergeFileDiffs_DoesNotMutateInputs(t *testing.T) {
	base := []FileDiff{
		{Path: "a.go", Hunks: []Hunk{{Header: "@@ base @@", Source: SourceBranch}}},
	}
	overlay := []FileDiff{
		{Path: "a.go", Hunks: []Hunk{{Header: "@@ overlay @@", Source: SourceStaged}}},
	}

	result := mergeFileDiffs(base, overlay)

	require.Len(t, result[0].Hunks, 2, "result should have combined hunks")
	require.Len(t, base[0].Hunks, 1, "base should not be mutated")
	require.Len(t, overlay[0].Hunks, 1, "overlay should not be mutated")
}

func TestMergeFileDiffs_EmptyBaseWithOverlay(t *testing.T) {
	overlay := []FileDiff{
		{Path: "a.go", Status: StatusAdded},
	}
	result := mergeFileDiffs(nil, overlay)
	require.Len(t, result, 1)
	assert.Equal(t, "a.go", result[0].Path)
}

func TestMergeFileDiffs_EmptyWT(t *testing.T) {
	branch := []FileDiff{
		{Path: "a.go", Status: StatusModified},
	}
	result := mergeFileDiffs(branch, nil)
	require.Len(t, result, 1)
	assert.Equal(t, "a.go", result[0].Path)
}

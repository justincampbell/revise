package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// tagHunks
// ============================================================================

func TestTagHunks_SetsSourceOnAllHunks(t *testing.T) {
	files := []FileDiff{
		{Path: "a.go", Hunks: []Hunk{{Header: "@@ -1 +1 @@"}, {Header: "@@ -2 +2 @@"}}},
		{Path: "b.go", Hunks: []Hunk{{Header: "@@ -1 +1 @@"}}},
	}
	tagHunks(files, SourceStaged)

	for _, f := range files {
		for _, h := range f.Hunks {
			assert.Equal(t, SourceStaged, h.Source)
		}
	}
}

func TestTagHunks_EmptyInput(t *testing.T) {
	tagHunks(nil, SourceBranch) // should not panic
}

// ============================================================================
// mergeFileDiffs — hunk overlap scenarios
//
// These tests document the current behavior when hunks from different sources
// cover overlapping line ranges. Deduplication is a future improvement — these
// tests pin the existing behavior so regressions are caught.
// ============================================================================

func TestMergeFileDiffs_OverlappingHunks_BranchAndStaged(t *testing.T) {
	// Branch modifies lines 1-5, staged also modifies lines 1-5.
	// Both hunks are kept (current behavior — no deduplication).
	branch := []FileDiff{{
		Path:   "a.go",
		Status: StatusModified,
		Hunks: []Hunk{{
			OldStart: 1, OldCount: 5, NewStart: 1, NewCount: 5,
			Header: "@@ -1,5 +1,5 @@", Source: SourceBranch,
			Lines: []Line{{Type: LineRemoved, Content: "old"}, {Type: LineAdded, Content: "branch"}},
		}},
	}}
	staged := []FileDiff{{
		Path:   "a.go",
		Status: StatusModified,
		Hunks: []Hunk{{
			OldStart: 1, OldCount: 5, NewStart: 1, NewCount: 5,
			Header: "@@ -1,5 +1,5 @@", Source: SourceStaged,
			Lines: []Line{{Type: LineRemoved, Content: "branch"}, {Type: LineAdded, Content: "staged"}},
		}},
	}}

	result := mergeFileDiffs(branch, staged)

	require.Len(t, result, 1)
	assert.Len(t, result[0].Hunks, 2, "overlapping hunks from different sources are both kept")
	assert.Equal(t, SourceBranch, result[0].Hunks[0].Source)
	assert.Equal(t, SourceStaged, result[0].Hunks[1].Source)
}

func TestMergeFileDiffs_OverlappingHunks_StagedAndUnstaged(t *testing.T) {
	// Staged and unstaged both modify the same region.
	staged := []FileDiff{{
		Path:   "a.go",
		Status: StatusModified,
		Hunks: []Hunk{{
			OldStart: 10, OldCount: 3, NewStart: 10, NewCount: 4,
			Header: "@@ -10,3 +10,4 @@", Source: SourceStaged,
		}},
	}}
	unstaged := []FileDiff{{
		Path:   "a.go",
		Status: StatusModified,
		Hunks: []Hunk{{
			OldStart: 10, OldCount: 4, NewStart: 10, NewCount: 3,
			Header: "@@ -10,4 +10,3 @@", Source: SourceUnstaged,
		}},
	}}

	result := mergeFileDiffs(staged, unstaged)

	require.Len(t, result, 1)
	assert.Len(t, result[0].Hunks, 2, "overlapping staged+unstaged hunks are both kept")
}

func TestMergeFileDiffs_OverlappingHunks_ThreeSources(t *testing.T) {
	// All three sources modify the same line range in a file.
	branch := []FileDiff{{
		Path:   "a.go",
		Status: StatusModified,
		Hunks: []Hunk{{
			OldStart: 1, OldCount: 3, NewStart: 1, NewCount: 3,
			Header: "@@ -1,3 +1,3 @@", Source: SourceBranch,
		}},
	}}
	staged := []FileDiff{{
		Path:   "a.go",
		Status: StatusModified,
		Hunks: []Hunk{{
			OldStart: 1, OldCount: 3, NewStart: 1, NewCount: 4,
			Header: "@@ -1,3 +1,4 @@", Source: SourceStaged,
		}},
	}}
	unstaged := []FileDiff{{
		Path:   "a.go",
		Status: StatusModified,
		Hunks: []Hunk{{
			OldStart: 1, OldCount: 4, NewStart: 1, NewCount: 3,
			Header: "@@ -1,4 +1,3 @@", Source: SourceUnstaged,
		}},
	}}

	// Compose: branch + working tree (staged + unstaged)
	wt := mergeFileDiffs(staged, unstaged)
	result := mergeFileDiffs(branch, wt)

	require.Len(t, result, 1)
	assert.Len(t, result[0].Hunks, 3, "all three overlapping hunks are kept")
}

func TestMergeFileDiffs_NewFileFromMultipleSources(t *testing.T) {
	// File added on branch, then staged changes to same file.
	// Both have OldStart=0 (new file hunks).
	branch := []FileDiff{{
		Path:   "new.go",
		Status: StatusAdded,
		Hunks: []Hunk{{
			OldStart: 0, OldCount: 0, NewStart: 1, NewCount: 5,
			Header: "@@ -0,0 +1,5 @@", Source: SourceBranch,
		}},
	}}
	staged := []FileDiff{{
		Path:   "new.go",
		Status: StatusModified,
		Hunks: []Hunk{{
			OldStart: 3, OldCount: 2, NewStart: 3, NewCount: 3,
			Header: "@@ -3,2 +3,3 @@", Source: SourceStaged,
		}},
	}}

	result := mergeFileDiffs(branch, staged)

	require.Len(t, result, 1)
	assert.Len(t, result[0].Hunks, 2, "branch add + staged edit hunks are both kept")
}

// ============================================================================
// mergeFileDiffs
// ============================================================================

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

func TestMergeFileDiffs_AddedThenDeleted(t *testing.T) {
	branch := []FileDiff{
		{Path: "temp.go", Status: StatusAdded},
		{Path: "keep.go", Status: StatusModified},
	}
	wt := []FileDiff{
		{Path: "temp.go", Status: StatusDeleted},
	}

	result := mergeFileDiffs(branch, wt)

	require.Len(t, result, 1, "temp.go should be removed (added then deleted = net zero)")
	assert.Equal(t, "keep.go", result[0].Path)
}

func TestMergeFileDiffs_ModifiedThenDeleted(t *testing.T) {
	branch := []FileDiff{
		{Path: "existing.go", Status: StatusModified, Hunks: []Hunk{{Header: "@@ branch @@", Source: SourceBranch}}},
	}
	wt := []FileDiff{
		{Path: "existing.go", Status: StatusDeleted, Hunks: []Hunk{{Header: "@@ delete @@", Source: SourceStaged}}},
	}

	result := mergeFileDiffs(branch, wt)

	require.Len(t, result, 1, "modified then deleted should still appear (file existed before branch)")
	require.Len(t, result[0].Hunks, 2, "should combine hunks from both")
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

// ============================================================================
// composeWorkingTree
// ============================================================================

func TestComposeWorkingTree_MergesAllSources(t *testing.T) {
	staged := []FileDiff{{Path: "a.go", Status: StatusModified, Hunks: []Hunk{{Header: "@@ staged @@"}}}}
	unstaged := []FileDiff{{Path: "b.go", Status: StatusModified, Hunks: []Hunk{{Header: "@@ unstaged @@"}}}}
	untracked := []FileDiff{{Path: "c.go", Status: StatusUntracked, Hunks: []Hunk{{Header: "@@ untracked @@", Source: SourceUnstaged}}}}

	result := composeWorkingTree(staged, unstaged, untracked)

	require.Len(t, result, 3)
	assert.Equal(t, SourceStaged, result[0].Hunks[0].Source)
	assert.Equal(t, SourceUnstaged, result[1].Hunks[0].Source)
	assert.Equal(t, SourceUnstaged, result[2].Hunks[0].Source)
}

func TestComposeWorkingTree_SameFileStagedAndUnstaged(t *testing.T) {
	staged := []FileDiff{{Path: "a.go", Status: StatusModified, Hunks: []Hunk{{Header: "@@ staged @@"}}}}
	unstaged := []FileDiff{{Path: "a.go", Status: StatusModified, Hunks: []Hunk{{Header: "@@ unstaged @@"}}}}

	result := composeWorkingTree(staged, unstaged, nil)

	require.Len(t, result, 1, "same file should appear once")
	require.Len(t, result[0].Hunks, 2)
	assert.Equal(t, SourceStaged, result[0].Hunks[0].Source)
	assert.Equal(t, SourceUnstaged, result[0].Hunks[1].Source)
}

func TestComposeWorkingTree_EmptyInputs(t *testing.T) {
	result := composeWorkingTree(nil, nil, nil)
	assert.Empty(t, result)
}

// ============================================================================
// composeBranch
// ============================================================================

func TestComposeBranch_MergesBranchAndWorkingTree(t *testing.T) {
	branch := []FileDiff{{Path: "a.go", Status: StatusAdded, Hunks: []Hunk{{Header: "@@ branch @@"}}}}
	wt := []FileDiff{{Path: "b.go", Status: StatusModified, Hunks: []Hunk{{Header: "@@ wt @@", Source: SourceStaged}}}}

	result := composeBranch(branch, wt)

	require.Len(t, result, 2)
	assert.Equal(t, SourceBranch, result[0].Hunks[0].Source)
	assert.Equal(t, SourceStaged, result[1].Hunks[0].Source)
}

func TestComposeBranch_SameFileCombinesHunks(t *testing.T) {
	branch := []FileDiff{{Path: "a.go", Status: StatusModified, Hunks: []Hunk{{Header: "@@ branch @@"}}}}
	wt := []FileDiff{{Path: "a.go", Status: StatusModified, Hunks: []Hunk{{Header: "@@ staged @@", Source: SourceStaged}}}}

	result := composeBranch(branch, wt)

	require.Len(t, result, 1)
	require.Len(t, result[0].Hunks, 2)
	assert.Equal(t, SourceBranch, result[0].Hunks[0].Source)
	assert.Equal(t, SourceStaged, result[0].Hunks[1].Source)
}

func TestComposeBranch_AddedThenDeletedNetZero(t *testing.T) {
	branch := []FileDiff{{Path: "temp.go", Status: StatusAdded, Hunks: []Hunk{{Header: "@@ branch @@"}}}}
	wt := []FileDiff{{Path: "temp.go", Status: StatusDeleted}}

	result := composeBranch(branch, wt)

	assert.Empty(t, result, "added on branch then deleted in working tree = net zero")
}

// ============================================================================
// mergeFileDiffs
// ============================================================================

func TestMergeFileDiffs_EmptyWT(t *testing.T) {
	branch := []FileDiff{
		{Path: "a.go", Status: StatusModified},
	}
	result := mergeFileDiffs(branch, nil)
	require.Len(t, result, 1)
	assert.Equal(t, "a.go", result[0].Path)
}

package git

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// IsGitRepo / HasCommits
// ============================================================================

func TestIsGitRepo_Inside(t *testing.T) {
	r := NewTestRepo(t)
	r.Chdir()
	assert.True(t, IsGitRepo())
}

func TestIsGitRepo_Outside(t *testing.T) {
	orig, _ := os.Getwd()
	os.Chdir(t.TempDir())
	t.Cleanup(func() { os.Chdir(orig) })
	assert.False(t, IsGitRepo())
}

func TestHasCommits_WithCommits(t *testing.T) {
	r := NewTestRepo(t)
	r.WriteFile("a.go", "package main\n")
	r.Add("a.go")
	r.Commit("first")
	r.Chdir()
	assert.True(t, HasCommits())
}

func TestHasCommits_EmptyRepo(t *testing.T) {
	r := NewTestRepo(t)
	r.Chdir()
	assert.False(t, HasCommits())
}

// ============================================================================
// StagedDiff / UnstagedDiff (raw function tests)
// ============================================================================

func TestStagedDiff_ContainsOnlyStagedContent(t *testing.T) {
	r := baseRepo(t)
	r.WriteFile("base.go", "package main\n\nfunc base() { /* ALPHA */ }\n")
	r.Add("base.go")
	r.WriteFile("base.go", "package main\n\nfunc base() { /* ALPHA */ }\n\n// BETA\n")

	raw, err := StagedDiff()
	require.NoError(t, err)
	assert.Contains(t, raw, "ALPHA")
	assert.NotContains(t, raw, "BETA")
}

func TestUnstagedDiff_ContainsUnstagedContent(t *testing.T) {
	r := baseRepo(t)
	r.WriteFile("base.go", "package main\n\nfunc base() { /* ALPHA */ }\n")
	r.Add("base.go")
	r.WriteFile("base.go", "package main\n\nfunc base() { /* ALPHA */ }\n\n// BETA\n")

	raw, err := UnstagedDiff()
	require.NoError(t, err)
	// BETA was added after staging; ALPHA may appear as context
	assert.Contains(t, raw, "BETA")
}

// ============================================================================
// GetDiff — on the default branch (merge-base == HEAD, shows working tree)
// ============================================================================

// Starting ref: default branch HEAD / Diff type: unstaged only
func TestGetDiff_DefaultBranch_UnstagedOnly(t *testing.T) {
	r := baseRepo(t)
	r.WriteFile("base.go", "package main\n\nfunc base() { /* unstaged */ }\n")

	diff, err := GetDiff()
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)
	assert.Equal(t, "base.go", diff.Files[0].Path)
	assert.Equal(t, StatusModified, diff.Files[0].Status)
	assertHasContent(t, diff.Files[0], "unstaged")
}

// Starting ref: default branch HEAD / Diff type: staged only
func TestGetDiff_DefaultBranch_StagedOnly(t *testing.T) {
	r := baseRepo(t)
	r.WriteFile("base.go", "package main\n\nfunc base() { /* staged */ }\n")
	r.Add("base.go")

	diff, err := GetDiff()
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)
	assertHasContent(t, diff.Files[0], "staged")
}

// Starting ref: default branch HEAD / Diff type: mix — staged new file + unstaged modification
func TestGetDiff_DefaultBranch_StagedAndUnstaged(t *testing.T) {
	r := baseRepo(t)
	r.WriteFile("staged.go", "package main\n\n// staged change\n")
	r.Add("staged.go")
	r.WriteFile("unstaged.go", "package main\n\n// unstaged change\n")

	diff, err := GetDiff()
	require.NoError(t, err)
	paths := filePaths(diff)
	assert.Contains(t, paths, "staged.go")
	assert.Contains(t, paths, "unstaged.go")
	assertHasContent(t, *fileByPath(diff, "staged.go"), "staged change")
}

// Starting ref: default branch HEAD / Diff type: mix — modified unstaged tracked file + new staged file
func TestGetDiff_DefaultBranch_ModifiedUnstagedAndNewStaged(t *testing.T) {
	r := baseRepo(t)
	r.WriteFile("new.go", "package main\n\nfunc newFn() {}\n")
	r.Add("new.go")
	r.WriteFile("base.go", "package main\n\nfunc base() { /* modified, not staged */ }\n")

	diff, err := GetDiff()
	require.NoError(t, err)
	paths := filePaths(diff)
	assert.Contains(t, paths, "new.go")
	assert.Contains(t, paths, "base.go")
}

// Starting ref: default branch HEAD / Untracked file
func TestGetDiff_DefaultBranch_UntrackedFile(t *testing.T) {
	r := baseRepo(t)
	r.WriteFile("untracked.go", "package main\n\nfunc untracked() {}\n")

	diff, err := GetDiff()
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)
	assert.Equal(t, StatusUntracked, diff.Files[0].Status)
	assert.Equal(t, "untracked.go", diff.Files[0].Path)
	assertHasContent(t, diff.Files[0], "untracked")
}

// ============================================================================
// GetDiff — on a feature branch (committed diff since merge-base + working tree)
// ============================================================================

// Starting ref: merge-base / Diff type: committed only, clean working tree
func TestGetDiff_FeatureBranch_CommittedOnly(t *testing.T) {
	_ = featureBranchRepo(t)

	diff, err := GetDiff()
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)
	assert.Equal(t, "feature.go", diff.Files[0].Path)
	assert.Equal(t, StatusAdded, diff.Files[0].Status)
}

// Starting ref: merge-base / Diff type: committed + unstaged modification to tracked file
func TestGetDiff_FeatureBranch_CommittedAndUnstaged(t *testing.T) {
	r := featureBranchRepo(t)
	r.WriteFile("base.go", "package main\n\nfunc base() { /* unstaged on branch */ }\n")

	diff, err := GetDiff()
	require.NoError(t, err)
	paths := filePaths(diff)
	assert.Contains(t, paths, "feature.go")
	assert.Contains(t, paths, "base.go")
	assertHasContent(t, *fileByPath(diff, "base.go"), "unstaged on branch")
}

// Starting ref: merge-base / Diff type: committed + staged new file
func TestGetDiff_FeatureBranch_CommittedAndStaged(t *testing.T) {
	r := featureBranchRepo(t)
	r.WriteFile("staged.go", "package main\n\nfunc staged() {}\n")
	r.Add("staged.go")

	diff, err := GetDiff()
	require.NoError(t, err)
	paths := filePaths(diff)
	assert.Contains(t, paths, "feature.go")
	assert.Contains(t, paths, "staged.go")
}

// Starting ref: merge-base / Diff type: committed + mix of staged and unstaged
func TestGetDiff_FeatureBranch_CommittedAndMixed(t *testing.T) {
	r := featureBranchRepo(t)
	r.WriteFile("staged.go", "package main\n\nfunc staged() {}\n")
	r.Add("staged.go")
	r.WriteFile("base.go", "package main\n\nfunc base() { /* unstaged */ }\n")

	diff, err := GetDiff()
	require.NoError(t, err)
	paths := filePaths(diff)
	assert.Contains(t, paths, "feature.go")
	assert.Contains(t, paths, "staged.go")
	assert.Contains(t, paths, "base.go")
}

// Working tree version of a committed file replaces the branch diff entry
func TestGetDiff_FeatureBranch_WorkingTreeOverridesCommitted(t *testing.T) {
	r := featureBranchRepo(t)
	r.WriteFile("feature.go", "package main\n\nfunc feature() { /* working tree */ }\n")
	r.Add("feature.go")

	diff, err := GetDiff()
	require.NoError(t, err)

	count := 0
	for _, f := range diff.Files {
		if f.Path == "feature.go" {
			count++
		}
	}
	assert.Equal(t, 1, count, "feature.go should appear exactly once")
	assertHasContent(t, *fileByPath(diff, "feature.go"), "working tree")
}

// Starting ref: merge-base / Everything: multiple commits + working tree changes
func TestGetDiff_FeatureBranch_MultipleCommitsAndWorkingTree(t *testing.T) {
	r := NewTestRepo(t)
	r.WriteFile("base.go", "package main\n\nfunc base() {}\n")
	r.Add("base.go")
	r.Commit("initial")
	r.CheckoutNewBranch("feature")

	r.WriteFile("one.go", "package main\n\nfunc one() {}\n")
	r.Add("one.go")
	r.Commit("add one")

	r.WriteFile("two.go", "package main\n\nfunc two() {}\n")
	r.Add("two.go")
	r.Commit("add two")

	r.WriteFile("wip.go", "package main\n\nfunc wip() {}\n")
	r.Chdir()

	diff, err := GetDiff()
	require.NoError(t, err)
	paths := filePaths(diff)
	assert.Contains(t, paths, "one.go")
	assert.Contains(t, paths, "two.go")
	assert.Contains(t, paths, "wip.go")
}

// ============================================================================
// GetDiff — deleted and renamed files
// ============================================================================

func TestGetDiff_DefaultBranch_DeletedFile(t *testing.T) {
	r := baseRepo(t)
	r.RemoveFile("base.go")
	r.Add("base.go")

	diff, err := GetDiff()
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)
	assert.Equal(t, StatusDeleted, diff.Files[0].Status)
}

func TestGetDiff_FeatureBranch_RenamedFile(t *testing.T) {
	r := NewTestRepo(t)
	r.WriteFile("old.go", "package main\n\nfunc old() {}\n")
	r.Add("old.go")
	r.Commit("initial")
	r.CheckoutNewBranch("feature")
	r.mustGit("mv", "old.go", "new.go")
	r.Commit("rename")
	r.Chdir()

	diff, err := GetDiff()
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)
	assert.Equal(t, StatusRenamed, diff.Files[0].Status)
	assert.Equal(t, "new.go", diff.Files[0].Path)
	assert.Equal(t, "old.go", diff.Files[0].OldPath)
}

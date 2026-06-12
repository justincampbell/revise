package git

import (
	"os"
	"os/exec"
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
	os.Chdir(t.TempDir())                //nolint:errcheck // test setup
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck // best-effort restore
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

	raw, err := StagedDiff(DefaultContextLines)
	require.NoError(t, err)
	assert.Contains(t, raw, "ALPHA")
	assert.NotContains(t, raw, "BETA")
}

func TestUnstagedDiff_ContainsUnstagedContent(t *testing.T) {
	r := baseRepo(t)
	r.WriteFile("base.go", "package main\n\nfunc base() { /* ALPHA */ }\n")
	r.Add("base.go")
	r.WriteFile("base.go", "package main\n\nfunc base() { /* ALPHA */ }\n\n// BETA\n")

	raw, err := UnstagedDiff(DefaultContextLines)
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

// Working tree version of a committed file combines hunks from both sources
func TestGetDiff_FeatureBranch_WorkingTreeCombinesWithCommitted(t *testing.T) {
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

	f := fileByPath(diff, "feature.go")
	require.NotNil(t, f)
	// Should have hunks from both branch and staged
	var sources []HunkSource
	for _, h := range f.Hunks {
		sources = append(sources, h.Source)
	}
	assert.Contains(t, sources, SourceBranch)
	assert.Contains(t, sources, SourceStaged)
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

// GetDiff uses origin/main (not local main) when a remote exists, so commits
// pushed to origin/main after the feature branch was created still show up.
func TestGetDiff_FeatureBranch_UsesOriginMain(t *testing.T) {
	// Create a bare repo to act as "origin".
	bare := t.TempDir()
	cmd := exec.Command("git", "init", "--bare", bare)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	// Ensure the bare repo's default branch is "main" regardless of git config.
	cmd = exec.Command("git", "-C", bare, "symbolic-ref", "HEAD", "refs/heads/main")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git symbolic-ref HEAD: %v\n%s", err, out)
	}

	r := NewTestRepo(t)
	r.WriteFile("base.go", "package main\n\nfunc base() {}\n")
	r.Add("base.go")
	r.Commit("initial")
	r.AddRemote(t, bare)

	// Create a feature branch and add a commit.
	r.CheckoutNewBranch("feature")
	r.WriteFile("feature.go", "package main\n\nfunc feature() {}\n")
	r.Add("feature.go")
	r.Commit("add feature")

	// Simulate origin/main moving ahead: push a new commit to origin/main
	// by cloning the bare repo into a second working copy.
	clone := t.TempDir()
	cmd = exec.Command("git", "clone", bare, clone)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}
	cloneR := &TestRepo{Dir: clone, t: t}
	cloneR.mustGit("config", "user.email", "test@example.com")
	cloneR.mustGit("config", "user.name", "Test")
	cloneR.WriteFile("upstream.go", "package main\n\nfunc upstream() {}\n")
	cloneR.Add("upstream.go")
	cloneR.Commit("upstream commit")
	cloneR.mustGit("push", "origin", "main")

	// Fetch so the original repo sees the new origin/main.
	r.mustGit("fetch", "origin")

	// Local main is still at the old commit, but origin/main has moved.
	// GetDiff should use origin/main, so feature.go shows up but upstream.go does not.
	r.Chdir()
	diff, err := GetDiff()
	require.NoError(t, err)
	paths := filePaths(diff)
	assert.Contains(t, paths, "feature.go")
	assert.NotContains(t, paths, "upstream.go", "should compare against origin/main, not local main")
}

// ============================================================================
// GetDiff — files added on branch then removed (net zero)
// ============================================================================

func TestGetDiff_FeatureBranch_AddedThenDeletedCommitted(t *testing.T) {
	r := NewTestRepo(t)
	r.WriteFile("base.go", "package main\n\nfunc base() {}\n")
	r.Add("base.go")
	r.Commit("initial")
	r.CheckoutNewBranch("feature")
	r.WriteFile("temp.go", "package main\n\nfunc temp() {}\n")
	r.Add("temp.go")
	r.Commit("add temp")
	r.mustGit("rm", "temp.go")
	r.Commit("remove temp")
	r.Chdir()

	diff, err := GetDiff()
	require.NoError(t, err)
	assert.Empty(t, diff.Files, "file added then removed in commits should not appear")
}

func TestGetDiff_FeatureBranch_AddedThenDeletedStaged(t *testing.T) {
	r := featureBranchRepo(t)
	r.mustGit("rm", "feature.go")

	diff, err := GetDiff()
	require.NoError(t, err)
	assert.Empty(t, diff.Files, "file added on branch then staged for deletion should not appear")
}

func TestGetDiff_FeatureBranch_AddedThenDeletedUnstaged(t *testing.T) {
	r := featureBranchRepo(t)
	r.RemoveFile("feature.go")

	diff, err := GetDiff()
	require.NoError(t, err)
	assert.Empty(t, diff.Files, "file added on branch then deleted from working tree should not appear")
}

func TestGetDiff_FeatureBranch_AddedThenRenamed(t *testing.T) {
	r := NewTestRepo(t)
	r.WriteFile("base.go", "package main\n\nfunc base() {}\n")
	r.Add("base.go")
	r.Commit("initial")
	r.CheckoutNewBranch("feature")
	r.WriteFile("temp.go", "package main\n\nfunc temp() {}\n")
	r.Add("temp.go")
	r.Commit("add temp")
	r.mustGit("mv", "temp.go", "renamed.go")
	r.Commit("rename temp to renamed")
	r.Chdir()

	diff, err := GetDiff()
	require.NoError(t, err)
	// The net effect is: renamed.go is a new file (temp.go never existed on main)
	paths := filePaths(diff)
	assert.NotContains(t, paths, "temp.go", "original name should not appear")
	assert.Contains(t, paths, "renamed.go", "renamed file should appear")
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

// ============================================================================
// RemoteName
// ============================================================================

func TestRemoteName_NoRemote(t *testing.T) {
	_ = baseRepo(t)
	assert.Equal(t, "", RemoteName())
}

func TestRemoteName_Origin(t *testing.T) {
	r := baseRepo(t)
	bare := t.TempDir()
	cmd := exec.Command("git", "init", "--bare", bare)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	r.AddRemote(t, bare)
	assert.Equal(t, "origin", RemoteName())
}

func TestRemoteName_NonOriginRemote(t *testing.T) {
	r := baseRepo(t)
	bare := t.TempDir()
	cmd := exec.Command("git", "init", "--bare", bare)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	_ = exec.Command("git", "-C", bare, "symbolic-ref", "HEAD", "refs/heads/main").Run()
	r.AddRemoteAs(t, bare, "upstream")
	assert.Equal(t, "upstream", RemoteName())
}

func TestRemoteName_PrefersOriginOverOthers(t *testing.T) {
	r := baseRepo(t)

	bare1 := t.TempDir()
	cmd := exec.Command("git", "init", "--bare", bare1)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	_ = exec.Command("git", "-C", bare1, "symbolic-ref", "HEAD", "refs/heads/main").Run()
	r.AddRemoteAs(t, bare1, "upstream")

	bare2 := t.TempDir()
	cmd = exec.Command("git", "init", "--bare", bare2)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	_ = exec.Command("git", "-C", bare2, "symbolic-ref", "HEAD", "refs/heads/main").Run()
	r.mustGit("remote", "add", "origin", bare2)

	assert.Equal(t, "origin", RemoteName())
}

// ============================================================================
// IsOnDefaultBranch
// ============================================================================

func TestIsOnDefaultBranch_OnMain(t *testing.T) {
	_ = baseRepo(t)
	onDefault, err := IsOnDefaultBranch()
	require.NoError(t, err)
	assert.True(t, onDefault)
}

func TestIsOnDefaultBranch_OnFeatureBranch(t *testing.T) {
	_ = featureBranchRepo(t)
	onDefault, err := IsOnDefaultBranch()
	require.NoError(t, err)
	assert.False(t, onDefault)
}

func TestIsOnDefaultBranch_FeatureBranchNoCommitsBehindRemote(t *testing.T) {
	r := behindRemoteRepo(t)
	r.CheckoutNewBranch("feature-no-commits")
	onDefault, err := IsOnDefaultBranch()
	require.NoError(t, err)
	assert.True(t, onDefault, "feature branch with no commits should be treated as default branch")
}

func TestIsOnDefaultBranch_EmptyRepo(t *testing.T) {
	r := NewTestRepo(t)
	r.Chdir()
	onDefault, err := IsOnDefaultBranch()
	require.NoError(t, err)
	assert.True(t, onDefault, "empty repo should be treated as default branch")
}

func TestIsOnDefaultBranch_BehindRemote(t *testing.T) {
	r := behindRemoteRepo(t)
	_ = r
	onDefault, err := IsOnDefaultBranch()
	require.NoError(t, err)
	assert.False(t, onDefault, "should return false when default branch is behind remote")
}

func TestIsOnDefaultBranch_AheadOfRemote(t *testing.T) {
	r := aheadOfRemoteRepo(t)
	_ = r
	onDefault, err := IsOnDefaultBranch()
	require.NoError(t, err)
	assert.False(t, onDefault, "should return false when default branch is ahead of remote")
}

// ============================================================================
// BranchDiff on default branch behind remote
// ============================================================================

func TestBranchDiff_DefaultBranchBehindRemote_ShowsRemoteChanges(t *testing.T) {
	_ = behindRemoteRepo(t)
	diff, err := BranchDiff(DefaultContextLines)
	require.NoError(t, err)
	paths := filePaths(diff)
	assert.Contains(t, paths, "remote-change.go", "should include file added on remote")
}

func TestBranchDiff_DefaultBranchAheadOfRemote_ShowsLocalChanges(t *testing.T) {
	_ = aheadOfRemoteRepo(t)
	diff, err := BranchDiff(DefaultContextLines)
	require.NoError(t, err)
	paths := filePaths(diff)
	assert.Contains(t, paths, "local-change.go", "should include locally committed file")
}

// ============================================================================
// Per-mode diff functions
// ============================================================================

func TestBranchDiff_ShowsEverything(t *testing.T) {
	r := featureBranchRepo(t)
	r.WriteFile("staged.go", "package main\n\n// staged\n")
	r.Add("staged.go")
	r.WriteFile("base.go", "package main\n\nfunc base() { /* unstaged */ }\n")
	r.WriteFile("untracked.go", "package main\n\n// untracked\n")

	diff, err := BranchDiff(DefaultContextLines)
	require.NoError(t, err)
	paths := filePaths(diff)
	assert.Contains(t, paths, "feature.go")
	assert.Contains(t, paths, "staged.go")
	assert.Contains(t, paths, "base.go")
	assert.Contains(t, paths, "untracked.go")
}

func TestStagedOnlyDiff_ExcludesUnstagedAndUntracked(t *testing.T) {
	r := baseRepo(t)
	r.WriteFile("staged.go", "package main\n\n// staged\n")
	r.Add("staged.go")
	r.WriteFile("base.go", "package main\n\nfunc base() { /* unstaged */ }\n")
	r.WriteFile("untracked.go", "package main\n\n// untracked\n")

	diff, err := StagedOnlyDiff(DefaultContextLines)
	require.NoError(t, err)
	paths := filePaths(diff)
	assert.Contains(t, paths, "staged.go")
	assert.NotContains(t, paths, "base.go")
	assert.NotContains(t, paths, "untracked.go")
}

func TestUnstagedOnlyDiff_ExcludesStaged(t *testing.T) {
	r := baseRepo(t)
	r.WriteFile("staged.go", "package main\n\n// staged\n")
	r.Add("staged.go")
	r.WriteFile("base.go", "package main\n\nfunc base() { /* unstaged */ }\n")
	r.WriteFile("untracked.go", "package main\n\n// untracked\n")

	diff, err := UnstagedOnlyDiff(DefaultContextLines)
	require.NoError(t, err)
	paths := filePaths(diff)
	assert.NotContains(t, paths, "staged.go")
	assert.Contains(t, paths, "base.go")
	assert.Contains(t, paths, "untracked.go")
}

func TestWorkingTreeDiff_ShowsStagedUnstagedUntracked(t *testing.T) {
	r := baseRepo(t)
	r.WriteFile("staged.go", "package main\n\n// staged\n")
	r.Add("staged.go")
	r.WriteFile("base.go", "package main\n\nfunc base() { /* unstaged */ }\n")
	r.WriteFile("untracked.go", "package main\n\n// untracked\n")

	diff, err := WorkingTreeDiff(DefaultContextLines)
	require.NoError(t, err)
	paths := filePaths(diff)
	assert.Contains(t, paths, "staged.go")
	assert.Contains(t, paths, "base.go")
	assert.Contains(t, paths, "untracked.go")
}

func TestWorkingTreeDiff_EmptyRepo_Untracked(t *testing.T) {
	r := NewTestRepo(t)
	r.Chdir()
	r.WriteFile("hello.go", "package main\n\nfunc main() {}\n")

	diff, err := WorkingTreeDiff(DefaultContextLines)
	require.NoError(t, err)
	paths := filePaths(diff)
	assert.Contains(t, paths, "hello.go")
}

func TestWorkingTreeDiff_EmptyRepo_Staged(t *testing.T) {
	r := NewTestRepo(t)
	r.Chdir()
	r.WriteFile("hello.go", "package main\n\nfunc main() {}\n")
	r.Add("hello.go")

	diff, err := WorkingTreeDiff(DefaultContextLines)
	require.NoError(t, err)
	paths := filePaths(diff)
	assert.Contains(t, paths, "hello.go")
	assertHasContent(t, *fileByPath(diff, "hello.go"), "func main")
}

func TestGetDiff_EmptyRepo(t *testing.T) {
	r := NewTestRepo(t)
	r.Chdir()
	r.WriteFile("hello.go", "package main\n")
	r.Add("hello.go")

	diff, err := GetDiff()
	require.NoError(t, err)
	assert.Contains(t, filePaths(diff), "hello.go")
}

// Same file with both staged and unstaged changes should appear once with hunks from both.
func TestWorkingTreeDiff_SameFileStagedAndUnstaged(t *testing.T) {
	r := baseRepo(t)
	r.WriteFile("base.go", "package main\n\nfunc base() { /* staged edit */ }\n")
	r.Add("base.go")
	r.WriteFile("base.go", "package main\n\nfunc base() { /* staged edit */ }\n\n// unstaged addition\n")

	diff, err := WorkingTreeDiff(DefaultContextLines)
	require.NoError(t, err)

	count := 0
	for _, f := range diff.Files {
		if f.Path == "base.go" {
			count++
		}
	}
	assert.Equal(t, 1, count, "base.go should appear exactly once")

	f := fileByPath(diff, "base.go")
	require.NotNil(t, f)
	// Should have hunks from both staged and unstaged
	assertHasContent(t, *f, "staged edit")
	assertHasContent(t, *f, "unstaged addition")

	// Verify hunk sources are tagged
	var sources []HunkSource
	for _, h := range f.Hunks {
		sources = append(sources, h.Source)
	}
	assert.Contains(t, sources, SourceStaged)
	assert.Contains(t, sources, SourceUnstaged)
}

// Same file changed across all three sources (branch + staged + unstaged) should combine all hunks.
func TestBranchDiff_ThreeSourceCombination(t *testing.T) {
	r := NewTestRepo(t)
	r.WriteFile("shared.go", "package main\n\nfunc shared() {}\n")
	r.Add("shared.go")
	r.Commit("initial")
	r.CheckoutNewBranch("feature")

	// Branch commit: modify the function
	r.WriteFile("shared.go", "package main\n\nfunc shared() { /* branch */ }\n")
	r.Add("shared.go")
	r.Commit("branch change")

	// Staged: add a new function
	r.WriteFile("shared.go", "package main\n\nfunc shared() { /* branch */ }\n\nfunc staged() {}\n")
	r.Add("shared.go")

	// Unstaged: add another line
	r.WriteFile("shared.go", "package main\n\nfunc shared() { /* branch */ }\n\nfunc staged() {}\n\n// unstaged\n")
	r.Chdir()

	diff, err := BranchDiff(DefaultContextLines)
	require.NoError(t, err)

	f := fileByPath(diff, "shared.go")
	require.NotNil(t, f)

	count := 0
	for _, fd := range diff.Files {
		if fd.Path == "shared.go" {
			count++
		}
	}
	assert.Equal(t, 1, count, "shared.go should appear exactly once")

	var sources []HunkSource
	for _, h := range f.Hunks {
		sources = append(sources, h.Source)
	}
	assert.Contains(t, sources, SourceBranch)
	assert.Contains(t, sources, SourceStaged)
	assert.Contains(t, sources, SourceUnstaged)
}

// When a file is modified on the branch and then staged with changes to the
// same lines, BranchDiff should show hunks from both sources. This documents
// the current overlap behavior (hunks are not deduplicated).
func TestBranchDiff_SameFileSameLinesBranchAndStaged(t *testing.T) {
	r := NewTestRepo(t)
	r.WriteFile("shared.go", "package main\n\nfunc shared() {\n\treturn\n}\n")
	r.Add("shared.go")
	r.Commit("initial")
	r.CheckoutNewBranch("feature")

	// Branch commit: modify the function body
	r.WriteFile("shared.go", "package main\n\nfunc shared() {\n\t// branch change\n\treturn\n}\n")
	r.Add("shared.go")
	r.Commit("branch change")

	// Staged: modify the same line again
	r.WriteFile("shared.go", "package main\n\nfunc shared() {\n\t// staged change\n\treturn\n}\n")
	r.Add("shared.go")
	r.Chdir()

	diff, err := BranchDiff(DefaultContextLines)
	require.NoError(t, err)

	f := fileByPath(diff, "shared.go")
	require.NotNil(t, f)

	// File should appear exactly once
	count := 0
	for _, fd := range diff.Files {
		if fd.Path == "shared.go" {
			count++
		}
	}
	assert.Equal(t, 1, count, "shared.go should appear exactly once")

	// Should have hunks from both Branch and Staged sources
	var sources []HunkSource
	for _, h := range f.Hunks {
		sources = append(sources, h.Source)
	}
	assert.Contains(t, sources, SourceBranch)
	assert.Contains(t, sources, SourceStaged)
}

// Untracked file hunks should be tagged as SourceUnstaged.
func TestUntrackedFiles_TaggedAsUnstaged(t *testing.T) {
	r := baseRepo(t)
	r.WriteFile("new.go", "package main\n\nfunc newFn() {}\n")

	diff, err := WorkingTreeDiff(DefaultContextLines)
	require.NoError(t, err)

	f := fileByPath(diff, "new.go")
	require.NotNil(t, f)
	require.Len(t, f.Hunks, 1)
	assert.Equal(t, SourceUnstaged, f.Hunks[0].Source)
}

func TestUntrackedFiles_BinaryDetected(t *testing.T) {
	r := baseRepo(t)
	// Write a file with null bytes (binary content)
	full := r.Dir + "/image.png"
	if err := os.WriteFile(full, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00}, 0644); err != nil {
		t.Fatal(err)
	}

	InvalidateUntrackedCache()
	files, err := UntrackedFiles()
	require.NoError(t, err)

	var found *FileDiff
	for i := range files {
		if files[i].Path == "image.png" {
			found = &files[i]
			break
		}
	}
	require.NotNil(t, found, "expected image.png in untracked files")
	assert.True(t, found.IsBinary, "expected image.png to be marked as binary")
	assert.Empty(t, found.Hunks, "binary files should have no hunks")
}

// ============================================================================
// Hide whitespace
// ============================================================================

func TestUnstagedDiff_HideWhitespace(t *testing.T) {
	r := baseRepo(t)
	// Add only whitespace changes
	r.WriteFile("base.go", "package main\n\nfunc base() {  }\n")

	raw, err := UnstagedDiff(DefaultContextLines)
	require.NoError(t, err)
	assert.NotEmpty(t, raw, "without -w, whitespace changes should appear")

	rawW, err := UnstagedDiffIgnoreWhitespace(DefaultContextLines)
	require.NoError(t, err)
	assert.Empty(t, rawW, "with -w, whitespace-only changes should be hidden")
}

func TestWorkingTreeDiff_HideWhitespace(t *testing.T) {
	r := baseRepo(t)
	// Only whitespace change
	r.WriteFile("base.go", "package main\n\nfunc base() {  }\n")

	diff, err := WorkingTreeDiffOptions(DefaultContextLines, true)
	require.NoError(t, err)
	assert.Empty(t, diff.Files, "whitespace-only changes should be hidden with -w")
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

// ============================================================================
// StatusFingerprint
// ============================================================================

func TestStatusFingerprint_ChangesOnNewFile(t *testing.T) {
	r := baseRepo(t)
	r.Chdir()

	fp1, err := StatusFingerprint()
	require.NoError(t, err)

	r.WriteFile("new.go", "package new\n")

	fp2, err := StatusFingerprint()
	require.NoError(t, err)

	assert.NotEqual(t, fp1, fp2)
}

func TestStatusFingerprint_ChangesOnStagedFile(t *testing.T) {
	r := baseRepo(t)
	r.Chdir()

	fp1, err := StatusFingerprint()
	require.NoError(t, err)

	r.WriteFile("base.go", "package main\n\nfunc base() { /* changed */ }\n")
	r.Add("base.go")

	fp2, err := StatusFingerprint()
	require.NoError(t, err)

	assert.NotEqual(t, fp1, fp2)
}

func TestStatusFingerprint_ChangesOnContentEdit(t *testing.T) {
	r := baseRepo(t)
	r.WriteFile("base.go", "package main\n\nfunc base() { /* v1 */ }\n")
	r.Chdir()

	fp1, err := StatusFingerprint()
	require.NoError(t, err)

	// Edit the same already-modified file again
	r.WriteFile("base.go", "package main\n\nfunc base() { /* v2 */ }\n")

	fp2, err := StatusFingerprint()
	require.NoError(t, err)

	assert.NotEqual(t, fp1, fp2, "content edits to already-modified files should change fingerprint")
}

func TestStatusFingerprint_StableWhenUnchanged(t *testing.T) {
	r := baseRepo(t)
	r.Chdir()

	fp1, err := StatusFingerprint()
	require.NoError(t, err)

	fp2, err := StatusFingerprint()
	require.NoError(t, err)

	assert.Equal(t, fp1, fp2)
}

// TestStatusFingerprint_ChangesOnCommit covers the case where stage+commit
// happens fast enough that the poll never sees the intermediate staged
// state. Without HEAD in the fingerprint, the pre-stage and post-commit
// status outputs are identical, so the auto-refresh loop wouldn't reload
// — and mode auto-promotion (ModeStaged → ModeBranch) would stall.
func TestStatusFingerprint_ChangesOnCommit(t *testing.T) {
	r := baseRepo(t)
	r.CheckoutNewBranch("feature")

	fp1, err := StatusFingerprint()
	require.NoError(t, err)

	r.WriteFile("new.go", "package new\n")
	r.Add("new.go")
	r.Commit("add new")

	fp2, err := StatusFingerprint()
	require.NoError(t, err)

	assert.NotEqual(t, fp1, fp2, "commit must change fingerprint even when working tree returns to clean")
}

// ============================================================================
// BranchDiffDepth / CommitsAhead
// ============================================================================

func TestCommitsAhead_CountsCommitsSinceMergeBase(t *testing.T) {
	r := NewTestRepo(t)
	r.WriteFile("base.go", "package main\n")
	r.Add("base.go")
	r.Commit("initial")
	r.CheckoutNewBranch("feature")

	r.WriteFile("one.go", "package main\n")
	r.Add("one.go")
	r.Commit("add one")
	r.WriteFile("two.go", "package main\n")
	r.Add("two.go")
	r.Commit("add two")
	r.Chdir()

	mergeBase, err := MergeBase("main")
	require.NoError(t, err)
	assert.Equal(t, 2, CommitsAhead(mergeBase))
}

func TestBranchCommits_ListsNewestFirst(t *testing.T) {
	r := NewTestRepo(t)
	r.WriteFile("base.go", "package main\n")
	r.Add("base.go")
	r.Commit("initial")
	r.CheckoutNewBranch("feature")

	r.WriteFile("one.go", "package main\n")
	r.Add("one.go")
	r.Commit("add one")
	r.WriteFile("two.go", "package main\n")
	r.Add("two.go")
	r.Commit("add two")
	r.Chdir()

	commits, err := BranchCommits()
	require.NoError(t, err)
	require.Len(t, commits, 2)
	assert.Equal(t, "add two", commits[0].Subject, "newest first")
	assert.Equal(t, "add one", commits[1].Subject)
	assert.NotEmpty(t, commits[0].ShortSHA)
	assert.NotEmpty(t, commits[0].SHA)
}

func TestBranchCommits_EmptyOnDefaultBranch(t *testing.T) {
	r := NewTestRepo(t)
	r.WriteFile("base.go", "package main\n")
	r.Add("base.go")
	r.Commit("initial")
	r.Chdir()

	commits, err := BranchCommits()
	require.NoError(t, err)
	assert.Empty(t, commits)
}

func TestBranchDiffDepth_LimitsToLastNCommits(t *testing.T) {
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
	r.WriteFile("three.go", "package main\n\nfunc three() {}\n")
	r.Add("three.go")
	r.Commit("add three")
	r.Chdir()

	// Depth 0 (full branch): 3 commits ahead, all three files.
	diff, ahead, err := BranchDiffDepth(DefaultContextLines, false, 0)
	require.NoError(t, err)
	assert.Equal(t, 3, ahead)
	paths := filePaths(diff)
	assert.Contains(t, paths, "one.go")
	assert.Contains(t, paths, "two.go")
	assert.Contains(t, paths, "three.go")

	// Depth 1: only the last commit (three.go).
	diff, ahead, err = BranchDiffDepth(DefaultContextLines, false, 1)
	require.NoError(t, err)
	assert.Equal(t, 3, ahead)
	paths = filePaths(diff)
	assert.NotContains(t, paths, "one.go")
	assert.NotContains(t, paths, "two.go")
	assert.Contains(t, paths, "three.go")

	// Depth 2: the last two commits (two.go, three.go).
	diff, _, err = BranchDiffDepth(DefaultContextLines, false, 2)
	require.NoError(t, err)
	paths = filePaths(diff)
	assert.NotContains(t, paths, "one.go")
	assert.Contains(t, paths, "two.go")
	assert.Contains(t, paths, "three.go")

	// Depth >= total clamps to the full branch.
	diff, _, err = BranchDiffDepth(DefaultContextLines, false, 99)
	require.NoError(t, err)
	paths = filePaths(diff)
	assert.Contains(t, paths, "one.go")
	assert.Contains(t, paths, "three.go")
}

// Working tree changes are always layered on top of the depth-limited range,
// so a depth-1 view shows the last commit plus uncommitted work.
func TestBranchDiffDepth_IncludesWorkingTree(t *testing.T) {
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

	// Uncommitted working tree file.
	r.WriteFile("wip.go", "package main\n\nfunc wip() {}\n")
	r.Chdir()

	diff, ahead, err := BranchDiffDepth(DefaultContextLines, false, 1)
	require.NoError(t, err)
	assert.Equal(t, 2, ahead)
	paths := filePaths(diff)
	assert.NotContains(t, paths, "one.go")
	assert.Contains(t, paths, "two.go")
	assert.Contains(t, paths, "wip.go")
}

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUnmergedPaths(t *testing.T) {
	out := "100644 aaa 1\tboth.go\x00" +
		"100644 bbb 2\tboth.go\x00" +
		"100644 ccc 3\tboth.go\x00" +
		"100644 ddd 3\tdir/with space.go\x00"

	assert.Equal(t, []string{"both.go", "dir/with space.go"}, parseUnmergedPaths(out))
	assert.Empty(t, parseUnmergedPaths(""))
}

func TestConflictedFiles_NoConflicts(t *testing.T) {
	r := baseRepo(t)
	r.WriteFile("base.go", "package main\n\n// changed\n")

	files, err := ConflictedFiles(DefaultContextLines, false)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestConflictedFiles_Rebase(t *testing.T) {
	r := divergedRepo(t)
	r.RebaseExpectConflict("main")

	files, err := ConflictedFiles(DefaultContextLines, false)
	require.NoError(t, err)
	require.Len(t, files, 1)

	f := files[0]
	assert.Equal(t, "base.go", f.Path)
	assert.Equal(t, StatusConflicted, f.Status)
	assertHasContent(t, f, "<<<<<<<")
	assertHasContent(t, f, "FEATURE")
	require.NotEmpty(t, f.Hunks)
	for _, h := range f.Hunks {
		assert.Equal(t, SourceUnstaged, h.Source)
	}
}

func TestConflictedFiles_ModifyDelete(t *testing.T) {
	r := NewTestRepo(t)
	r.WriteFile("gone.go", "package main\n")
	r.Add("gone.go")
	r.Commit("initial")
	r.CheckoutNewBranch("feature")
	r.WriteFile("gone.go", "package main\n\n// FEATURE\n")
	r.Add("gone.go")
	r.Commit("modify")
	r.mustGit("checkout", "main")
	r.mustGit("rm", "-q", "gone.go")
	r.Commit("delete")
	r.mustGit("checkout", "feature")
	r.Chdir()
	r.RebaseExpectConflict("main")

	files, err := ConflictedFiles(DefaultContextLines, false)
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "gone.go", files[0].Path)
	assert.Equal(t, StatusConflicted, files[0].Status)
}

func TestWorkingTreeDiff_RebaseConflict(t *testing.T) {
	r := divergedRepo(t)
	r.RebaseExpectConflict("main")

	diff, err := WorkingTreeDiff(DefaultContextLines)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"base.go", "other.go"}, filePaths(diff))

	conflicted := fileByPath(diff, "base.go")
	require.NotNil(t, conflicted)
	assert.Equal(t, StatusConflicted, conflicted.Status)
	assertHasContent(t, *conflicted, "<<<<<<<")

	// The cleanly applied file is unaffected and doesn't pick up the
	// combined-diff lines of the conflicted one.
	clean := fileByPath(diff, "other.go")
	require.NotNil(t, clean)
	assert.Equal(t, StatusModified, clean.Status)
	for _, h := range clean.Hunks {
		for _, l := range h.Lines {
			assert.NotContains(t, l.Content, "<<<<<<<")
		}
	}
}

func TestUnstagedOnlyDiff_IncludesConflicted(t *testing.T) {
	r := divergedRepo(t)
	r.RebaseExpectConflict("main")

	diff, err := UnstagedOnlyDiff(DefaultContextLines)
	require.NoError(t, err)
	f := fileByPath(diff, "base.go")
	require.NotNil(t, f)
	assert.Equal(t, StatusConflicted, f.Status)
}

func TestStagedOnlyDiff_ExcludesConflicted(t *testing.T) {
	r := divergedRepo(t)
	r.RebaseExpectConflict("main")

	diff, err := StagedOnlyDiff(DefaultContextLines)
	require.NoError(t, err)
	assert.Equal(t, []string{"other.go"}, filePaths(diff))
}

func TestBranchDiff_MergeConflict(t *testing.T) {
	r := divergedRepo(t)
	r.MergeExpectConflict("main")

	diff, err := BranchDiff(DefaultContextLines)
	require.NoError(t, err)
	f := fileByPath(diff, "base.go")
	require.NotNil(t, f)
	assert.Equal(t, StatusConflicted, f.Status)
	assertHasContent(t, *f, "<<<<<<<")
}

func TestConflictedFiles_ResolvedByStaging(t *testing.T) {
	r := divergedRepo(t)
	r.RebaseExpectConflict("main")
	r.WriteFile("base.go", "package main\n\nfunc base() { /* RESOLVED */ }\n")
	r.Add("base.go")

	files, err := ConflictedFiles(DefaultContextLines, false)
	require.NoError(t, err)
	assert.Empty(t, files)
}

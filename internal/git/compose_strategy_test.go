package git

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRangesIntersect(t *testing.T) {
	tests := []struct {
		name string
		a, b []interval
		want bool
	}{
		{"empty both", nil, nil, false},
		{"empty a", nil, []interval{{1, 5}}, false},
		{"disjoint before", []interval{{1, 3}}, []interval{{5, 8}}, false},
		{"disjoint after", []interval{{10, 12}}, []interval{{5, 8}}, false},
		{"adjacent half-open (no overlap)", []interval{{1, 5}}, []interval{{5, 8}}, false},
		{"overlap by one", []interval{{1, 6}}, []interval{{5, 8}}, true},
		{"contained", []interval{{1, 10}}, []interval{{4, 6}}, true},
		{"identical", []interval{{2, 4}}, []interval{{2, 4}}, true},
		{"second pair overlaps", []interval{{1, 3}, {20, 25}}, []interval{{5, 8}, {22, 30}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, rangesIntersect(tt.a, tt.b))
		})
	}
}

func TestNewAndOldSideRanges(t *testing.T) {
	files := []FileDiff{{
		Path: "a.txt",
		Hunks: []Hunk{
			{OldStart: 3, OldCount: 4, NewStart: 3, NewCount: 5},
			{OldStart: 20, OldCount: 0, NewStart: 21, NewCount: 2},
		},
	}}

	newR := newSideRanges(files)
	assert.Equal(t, []interval{{3, 8}, {21, 23}}, newR["a.txt"])

	oldR := oldSideRanges(files)
	// OldCount 0 (pure insertion) still yields a width-1 interval so adjacency
	// can be detected.
	assert.Equal(t, []interval{{3, 7}, {20, 21}}, oldR["a.txt"])
}

// numberedFile returns "line 1\nline 2\n...\nline n\n".
func numberedFile(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

// replaceLine returns content with the 1-indexed line replaced by repl.
func replaceLine(content string, line int, repl string) string {
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	lines[line-1] = repl
	return strings.Join(lines, "\n") + "\n"
}

func diffHasLine(f *FileDiff, typ LineType, content string) bool {
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			if l.Type == typ && l.Content == content {
				return true
			}
		}
	}
	return false
}

// sameLineRepo: main commit, feature commit changing line 2, then an unstaged
// change to the same line 2. This is the phantom-intermediate scenario.
func sameLineRepo(t *testing.T) *TestRepo {
	t.Helper()
	base := numberedFile(25)
	r := NewTestRepo(t)
	r.WriteFile("file.txt", base)
	r.Add("file.txt")
	r.Commit("initial on main")
	r.CheckoutNewBranch("feature")
	r.WriteFile("file.txt", replaceLine(base, 2, "line 2 (branch)"))
	r.Add("file.txt")
	r.Commit("change line 2 on branch")
	r.WriteFile("file.txt", replaceLine(base, 2, "line 2 (unstaged)"))
	r.Chdir()
	return r
}

func TestBranchDiff_Stack_SameLineShowsIntermediate(t *testing.T) {
	sameLineRepo(t)

	diff, _, err := BranchDiffDepth(DefaultContextLines, false, 0, false)
	require.NoError(t, err)
	f := fileByPath(diff, "file.txt")
	require.NotNil(t, f)

	require.Len(t, f.Hunks, 2, "stack keeps committed and unstaged hunks separate")
	assert.Equal(t, SourceBranch, f.Hunks[0].Source)
	assert.Equal(t, SourceUnstaged, f.Hunks[1].Source)
	// The phantom intermediate: "line 2 (branch)" appears as a removed line.
	assert.True(t, diffHasLine(f, LineRemoved, "line 2 (branch)"),
		"stack exposes the intermediate branch state as the unstaged hunk's base")
}

func TestBranchDiff_Overlap_SameLineCollapses(t *testing.T) {
	sameLineRepo(t)

	diff, _, err := BranchDiffDepth(DefaultContextLines, false, 0, true)
	require.NoError(t, err)
	f := fileByPath(diff, "file.txt")
	require.NotNil(t, f)

	require.Len(t, f.Hunks, 1, "overlapping ranges collapse to the final state")
	assert.Equal(t, SourceOverlap, f.Hunks[0].Source, "collapsed hunks are tagged SourceOverlap")
	assert.True(t, diffHasLine(f, LineRemoved, "line 2"))
	assert.True(t, diffHasLine(f, LineAdded, "line 2 (unstaged)"))
	assert.False(t, diffHasLine(f, LineRemoved, "line 2 (branch)"), "no phantom intermediate")
	assert.False(t, diffHasLine(f, LineAdded, "line 2 (branch)"), "no phantom intermediate")
}

func TestBranchDiff_Overlap_DisjointLinesKeepsStack(t *testing.T) {
	base := numberedFile(25)
	r := NewTestRepo(t)
	r.WriteFile("file.txt", base)
	r.Add("file.txt")
	r.Commit("initial on main")
	r.CheckoutNewBranch("feature")
	r.WriteFile("file.txt", replaceLine(base, 2, "line 2 (branch)"))
	r.Add("file.txt")
	r.Commit("change line 2 on branch")
	// Unstaged change far from the committed change — ranges do not overlap.
	r.WriteFile("file.txt", replaceLine(replaceLine(base, 2, "line 2 (branch)"), 20, "line 20 (unstaged)"))
	r.Chdir()

	diff, _, err := BranchDiffDepth(DefaultContextLines, false, 0, true)
	require.NoError(t, err)
	f := fileByPath(diff, "file.txt")
	require.NotNil(t, f)

	require.Len(t, f.Hunks, 2, "disjoint edits keep the tagged stack")
	assert.Equal(t, SourceBranch, f.Hunks[0].Source)
	assert.Equal(t, SourceUnstaged, f.Hunks[1].Source)
}

// TestBranchDiff_Overlap_NoSharedFilesLeavesTags covers the optimization: when
// no file changed both in commits and in the working tree, overlap mode leaves
// every file's source tags intact (and does no collapsing).
func TestBranchDiff_Overlap_NoSharedFilesLeavesTags(t *testing.T) {
	base := numberedFile(10)
	r := NewTestRepo(t)
	r.WriteFile("committed.txt", base)
	r.Add("committed.txt")
	r.Commit("initial on main")
	r.CheckoutNewBranch("feature")
	// committed.txt changes only on the branch.
	r.WriteFile("committed.txt", replaceLine(base, 3, "line 3 (branch)"))
	r.Add("committed.txt")
	r.Commit("change committed.txt on branch")
	// working.txt is a brand-new untracked file — never committed.
	r.WriteFile("working.txt", "hello\n")
	r.Chdir()

	diff, _, err := BranchDiffDepth(DefaultContextLines, false, 0, true)
	require.NoError(t, err)

	committed := fileByPath(diff, "committed.txt")
	require.NotNil(t, committed)
	require.Len(t, committed.Hunks, 1)
	assert.Equal(t, SourceBranch, committed.Hunks[0].Source, "branch-only file keeps its tag")

	working := fileByPath(diff, "working.txt")
	require.NotNil(t, working)
	for _, h := range working.Hunks {
		assert.NotEqual(t, SourceOverlap, h.Source, "untracked file is never collapsed")
	}
}

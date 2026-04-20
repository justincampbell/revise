package git

import (
	"fmt"
	"testing"
)

func makeFileDiffs(n int, hunksPerFile int, source HunkSource) []FileDiff {
	files := make([]FileDiff, n)
	for i := range files {
		hunks := make([]Hunk, hunksPerFile)
		for j := range hunks {
			hunks[j] = Hunk{
				OldStart: j*10 + 1,
				OldCount: 5,
				NewStart: j*10 + 1,
				NewCount: 6,
				Header:   fmt.Sprintf("@@ -%d,5 +%d,6 @@", j*10+1, j*10+1),
				Source:   source,
			}
		}
		files[i] = FileDiff{
			Path:   fmt.Sprintf("file%d.go", i),
			Status: StatusModified,
			Hunks:  hunks,
		}
	}
	return files
}

func BenchmarkMergeFileDiffs_Small(b *testing.B) {
	base := makeFileDiffs(10, 3, SourceBranch)
	overlay := makeFileDiffs(10, 2, SourceStaged)

	b.ResetTimer()
	for b.Loop() {
		mergeFileDiffs(base, overlay)
	}
}

func BenchmarkMergeFileDiffs_Large(b *testing.B) {
	base := makeFileDiffs(100, 10, SourceBranch)
	overlay := makeFileDiffs(100, 5, SourceStaged)

	b.ResetTimer()
	for b.Loop() {
		mergeFileDiffs(base, overlay)
	}
}

func BenchmarkMergeFileDiffs_DisjointFiles(b *testing.B) {
	base := makeFileDiffs(50, 5, SourceBranch)
	// Different file names — no merging, just appends
	overlay := make([]FileDiff, 50)
	for i := range overlay {
		overlay[i] = FileDiff{
			Path:   fmt.Sprintf("other%d.go", i),
			Status: StatusAdded,
			Hunks:  []Hunk{{Header: "@@ -0,0 +1,1 @@", Source: SourceStaged}},
		}
	}

	b.ResetTimer()
	for b.Loop() {
		mergeFileDiffs(base, overlay)
	}
}

func BenchmarkComposeWorkingTree(b *testing.B) {
	staged := makeFileDiffs(20, 3, SourceStaged)
	unstaged := makeFileDiffs(20, 2, SourceUnstaged)
	untracked := makeFileDiffs(10, 1, SourceUnstaged)
	for i := range untracked {
		untracked[i].Status = StatusUntracked
	}

	b.ResetTimer()
	for b.Loop() {
		composeWorkingTree(staged, unstaged, untracked)
	}
}

package git

import (
	"fmt"
	"os/exec"
)

// composeOverlap keeps the tagged stack (committed hunks then working-tree
// hunks, each labeled by source) for files whose committed and working-tree
// edits touch disjoint line ranges, but substitutes a single collapsed
// merge-base→worktree diff for files where the ranges overlap — the case where
// stacking would otherwise expose a misleading intermediate state that never
// exists on disk. Collapsed hunks are tagged SourceOverlap.
//
// `from` is the committed range's base ref (the merge-base, or HEAD~depth when
// the commit filter is active); it is reused to recompute the collapsed diff.
//
// Cost is proportional to the overlap-candidate set: only files changed in both
// the committed range and the working tree can overlap, so when there are none
// this returns the plain stack with no extra git calls, and otherwise the two
// helper diffs are scoped to just the candidate paths.
func composeOverlap(branch, workingTree []FileDiff, from string, contextLines int, hideWhitespace bool) []FileDiff {
	tagHunks(branch, SourceBranch)
	stacked := mergeFileDiffs(branch, workingTree)

	// A file can only overlap if it changed in both the committed range and the
	// working tree. Untracked files are never in `branch`, so they drop out here.
	branchPaths := pathSet(branch)
	var candidates []string
	for _, f := range workingTree {
		if branchPaths[f.Path] {
			candidates = append(candidates, f.Path)
		}
	}
	if len(candidates) == 0 {
		return stacked
	}

	// Ranges are compared in HEAD coordinates: the committed range's new side
	// and `git diff HEAD`'s old side both index HEAD line numbers.
	branchRanges := newSideRanges(branch)
	var headRanges map[string][]interval
	if headRaw, err := rawDiffHead(contextLines, hideWhitespace, candidates); err == nil {
		headRanges = oldSideRanges(Parse(headRaw).Files)
	}

	collapsed := make(map[string]FileDiff)
	if raw, err := rawDiffFromWorktree(from, contextLines, hideWhitespace, candidates); err == nil {
		for _, f := range Parse(raw).Files {
			collapsed[f.Path] = f
		}
	}

	for i := range stacked {
		path := stacked[i].Path
		if !branchPaths[path] || !rangesIntersect(branchRanges[path], headRanges[path]) {
			continue
		}
		cf, ok := collapsed[path]
		if !ok {
			continue
		}
		for j := range cf.Hunks {
			cf.Hunks[j].Source = SourceOverlap
		}
		stacked[i].Hunks = cf.Hunks
	}
	return stacked
}

// pathSet returns the set of file paths in files.
func pathSet(files []FileDiff) map[string]bool {
	m := make(map[string]bool, len(files))
	for _, f := range files {
		m[f.Path] = true
	}
	return m
}

// interval is a half-open line range [start, end).
type interval struct{ start, end int }

// newSideRanges collects each file's new-side hunk ranges (the "+" side line
// numbers). For a committed (merge-base→HEAD) diff these index HEAD lines.
func newSideRanges(files []FileDiff) map[string][]interval {
	m := make(map[string][]interval, len(files))
	for _, f := range files {
		for _, h := range f.Hunks {
			m[f.Path] = append(m[f.Path], interval{h.NewStart, h.NewStart + max(h.NewCount, 1)})
		}
	}
	return m
}

// oldSideRanges collects each file's old-side hunk ranges (the "-" side line
// numbers). For a `git diff HEAD` these index HEAD lines.
func oldSideRanges(files []FileDiff) map[string][]interval {
	m := make(map[string][]interval, len(files))
	for _, f := range files {
		for _, h := range f.Hunks {
			m[f.Path] = append(m[f.Path], interval{h.OldStart, h.OldStart + max(h.OldCount, 1)})
		}
	}
	return m
}

// rangesIntersect reports whether any interval in a overlaps any interval in b.
func rangesIntersect(a, b []interval) bool {
	for _, x := range a {
		for _, y := range b {
			if x.start < y.end && y.start < x.end {
				return true
			}
		}
	}
	return false
}

// rawDiffFromWorktree returns `git diff <from> [-- paths...]` — the diff from a
// commit ref to the current working tree (staged + unstaged, tracked files).
func rawDiffFromWorktree(from string, contextLines int, hideWhitespace bool, paths []string) (string, error) {
	args := []string{"diff"}
	if hideWhitespace {
		args = append(args, "-w")
	}
	args = append(args, fmt.Sprintf("-U%d", contextLines), from)
	args = appendPathspec(args, paths)
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git diff %s failed: %s", from, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git diff %s: %w", from, err)
	}
	return string(out), nil
}

// rawDiffHead returns `git diff HEAD [-- paths...]` — working tree (staged +
// unstaged) vs HEAD. Its old side indexes HEAD lines, matching newSideRanges'
// coordinates.
func rawDiffHead(contextLines int, hideWhitespace bool, paths []string) (string, error) {
	args := []string{"diff"}
	if hideWhitespace {
		args = append(args, "-w")
	}
	args = append(args, fmt.Sprintf("-U%d", contextLines), "HEAD")
	args = appendPathspec(args, paths)
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git diff HEAD failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git diff HEAD: %w", err)
	}
	return string(out), nil
}

// appendPathspec appends a `-- path...` pathspec when paths is non-empty.
func appendPathspec(args, paths []string) []string {
	if len(paths) == 0 {
		return args
	}
	args = append(args, "--")
	return append(args, paths...)
}

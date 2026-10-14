package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// ConflictedFiles returns a FileDiff for each unmerged path (left behind by a
// rebase, merge, cherry-pick, etc. that stopped on conflicts), with status
// StatusConflicted.
//
// `git diff` renders unmerged paths as combined (`diff --cc`) diffs, which
// don't fit the two-sided Line model, so each path is instead diffed from HEAD
// to the working tree: the conflict markers show up as added lines, with
// ordinary line numbers that comments can anchor to. Hunks are tagged
// SourceUnstaged, since resolution happens in the working tree. A path with
// nothing to diff (e.g. deleted in the working tree and absent from HEAD)
// still gets an entry with no hunks, so the conflict is never hidden.
func ConflictedFiles(contextLines int, hideWhitespace bool) ([]FileDiff, error) {
	paths, err := unmergedPaths()
	if err != nil || len(paths) == 0 {
		return nil, err
	}

	args := []string{"diff", "--no-renames", fmt.Sprintf("-U%d", contextLines)}
	if hideWhitespace {
		args = append(args, "-w")
	}
	args = append(args, "HEAD", "--")
	args = append(args, paths...)
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff HEAD (conflicts): %w", err)
	}

	byPath := make(map[string]FileDiff)
	for _, f := range Parse(string(out)).Files {
		byPath[f.Path] = f
	}

	files := make([]FileDiff, 0, len(paths))
	for _, p := range paths {
		f, ok := byPath[p]
		if !ok {
			f = FileDiff{Path: p}
		}
		f.Status = StatusConflicted
		files = append(files, f)
	}
	tagHunks(files, SourceUnstaged)
	return files, nil
}

// unmergedPaths lists paths with unresolved conflicts in the index. It reads
// only the index (no working tree scan), so it's cheap to run on every refresh.
func unmergedPaths() ([]string, error) {
	out, err := exec.Command("git", "ls-files", "--unmerged", "-z").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files --unmerged: %w", err)
	}
	return parseUnmergedPaths(string(out)), nil
}

// parseUnmergedPaths extracts unique paths, in order, from
// `git ls-files --unmerged -z` output: one "<mode> <sha> <stage>\t<path>"
// record per conflict stage (up to three per path).
func parseUnmergedPaths(out string) []string {
	var paths []string
	seen := make(map[string]bool)
	for _, rec := range strings.Split(out, "\x00") {
		_, p, ok := strings.Cut(rec, "\t")
		if !ok || p == "" || seen[p] {
			continue
		}
		seen[p] = true
		paths = append(paths, p)
	}
	return paths
}

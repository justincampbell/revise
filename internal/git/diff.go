package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// DefaultBranch detects the default branch of the repository.
func DefaultBranch() (string, error) {
	// Try to get the default branch from the remote
	out, err := exec.Command("git", "rev-parse", "--verify", "--quiet", "refs/remotes/origin/main").Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return "main", nil
	}

	out, err = exec.Command("git", "rev-parse", "--verify", "--quiet", "refs/remotes/origin/master").Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return "master", nil
	}

	// Fallback: check remote HEAD
	out, err = exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD").Output()
	if err == nil {
		ref := strings.TrimSpace(string(out))
		// refs/remotes/origin/main -> main
		parts := strings.Split(ref, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}

	return "main", nil
}

// MergeBase returns the merge base between the current HEAD and the given branch.
func MergeBase(branch string) (string, error) {
	out, err := exec.Command("git", "merge-base", branch, "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("finding merge-base with %s: %w", branch, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RawDiff returns the raw unified diff from the merge-base to HEAD.
func RawDiff(mergeBase string) (string, error) {
	out, err := exec.Command("git", "diff", mergeBase, "HEAD").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git diff failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

// StagedDiff returns the raw diff of staged changes.
func StagedDiff() (string, error) {
	out, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil {
		return "", fmt.Errorf("git diff --cached: %w", err)
	}
	return string(out), nil
}

// UnstagedDiff returns the raw diff of unstaged changes.
func UnstagedDiff() (string, error) {
	out, err := exec.Command("git", "diff").Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

// IsGitRepo checks if the current directory is inside a git repository.
func IsGitRepo() bool {
	err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Run()
	return err == nil
}

// HasCommits checks if the repository has any commits.
func HasCommits() bool {
	err := exec.Command("git", "rev-parse", "HEAD").Run()
	return err == nil
}

// CurrentRef returns the current HEAD commit hash.
func CurrentRef() (string, error) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// IsOnDefaultBranch returns true if the current HEAD is at the merge-base
// with the default branch (i.e. no feature-branch commits).
func IsOnDefaultBranch() (bool, error) {
	branch, err := DefaultBranch()
	if err != nil {
		return false, err
	}

	remoteBranch := "origin/" + branch
	mergeBase, err := MergeBase(remoteBranch)
	if err != nil {
		mergeBase, err = MergeBase(branch)
	}
	if err != nil {
		// No merge-base means we can't determine — treat as default branch
		return true, nil
	}

	head, err := CurrentRef()
	if err != nil {
		return false, err
	}

	return mergeBase == head, nil
}

// resolveMergeBase finds the merge-base for the current branch.
func resolveMergeBase() (string, error) {
	branch, err := DefaultBranch()
	if err != nil {
		return "", err
	}

	remoteBranch := "origin/" + branch
	mergeBase, err := MergeBase(remoteBranch)
	if err != nil {
		mergeBase, err = MergeBase(branch)
	}
	if err != nil {
		return "", fmt.Errorf("no merge-base found: %w", err)
	}
	return mergeBase, nil
}

// BranchDiff returns the merge-base diff merged with all working tree changes.
// This is the broadest view — committed + staged + unstaged + untracked.
func BranchDiff() (*Diff, error) {
	mergeBase, err := resolveMergeBase()
	if err != nil {
		return nil, err
	}
	raw, err := RawDiff(mergeBase)
	if err != nil {
		return nil, err
	}
	diff := Parse(raw)
	tagHunks(diff.Files, SourceBranch)

	wtDiff, err := WorkingTreeDiff()
	if err != nil {
		return nil, err
	}
	// WorkingTreeDiff already tags its hunks as Staged/Unstaged
	diff.Files = mergeFileDiffs(diff.Files, wtDiff.Files)
	return diff, nil
}

// StagedOnlyDiff returns only staged changes.
func StagedOnlyDiff() (*Diff, error) {
	raw, err := StagedDiff()
	if err != nil {
		return nil, err
	}
	diff := Parse(raw)
	tagHunks(diff.Files, SourceStaged)
	return diff, nil
}

// UnstagedOnlyDiff returns unstaged changes + untracked files.
func UnstagedOnlyDiff() (*Diff, error) {
	raw, err := UnstagedDiff()
	if err != nil {
		return nil, err
	}

	diff := Parse(raw)
	tagHunks(diff.Files, SourceUnstaged)

	untracked, err := UntrackedFiles()
	if err != nil {
		return nil, err
	}
	diff.Files = append(diff.Files, untracked...)

	return diff, nil
}

// WorkingTreeDiff returns staged + unstaged + untracked changes.
func WorkingTreeDiff() (*Diff, error) {
	return getWorkingTreeDiff()
}

// GetDiff returns a parsed Diff for the current branch vs the default branch.
// If on the default branch, it shows working tree changes (staged + unstaged).
// Otherwise it shows committed changes vs merge-base plus working tree changes.
func GetDiff() (*Diff, error) {
	if !IsGitRepo() {
		return nil, fmt.Errorf("not a git repository")
	}

	if !HasCommits() {
		return nil, fmt.Errorf("repository has no commits")
	}

	onDefault, err := IsOnDefaultBranch()
	if err != nil {
		return nil, err
	}

	if onDefault {
		return WorkingTreeDiff()
	}

	return BranchDiff()
}

// getWorkingTreeDiff returns staged + unstaged + untracked changes.
func getWorkingTreeDiff() (*Diff, error) {
	stagedRaw, err := StagedDiff()
	if err != nil {
		return nil, err
	}
	unstagedRaw, err := UnstagedDiff()
	if err != nil {
		return nil, err
	}

	staged := Parse(stagedRaw)
	tagHunks(staged.Files, SourceStaged)
	unstaged := Parse(unstagedRaw)
	tagHunks(unstaged.Files, SourceUnstaged)
	// Merge so same-path files appear once with hunks from both.
	staged.Files = mergeFileDiffs(staged.Files, unstaged.Files)

	untracked, err := UntrackedFiles()
	if err != nil {
		return nil, err
	}
	staged.Files = append(staged.Files, untracked...)

	return staged, nil
}

// tagHunks sets the Source field on all hunks in the given file diffs.
func tagHunks(files []FileDiff, source HunkSource) {
	for i := range files {
		for j := range files[i].Hunks {
			files[i].Hunks[j].Source = source
		}
	}
}

// mergeFileDiffs combines two sets of file diffs.
// For the same path, hunks from both are combined (base first, then overlay).
// New paths are appended. The inputs are not mutated.
func mergeFileDiffs(base, overlay []FileDiff) []FileDiff {
	result := make([]FileDiff, len(base))
	copy(result, base)

	seen := make(map[string]int, len(result))
	for i, f := range result {
		seen[f.Path] = i
	}

	for _, f := range overlay {
		if idx, ok := seen[f.Path]; ok {
			// Same file — combine hunks
			combined := make([]Hunk, len(result[idx].Hunks), len(result[idx].Hunks)+len(f.Hunks))
			copy(combined, result[idx].Hunks)
			combined = append(combined, f.Hunks...)
			result[idx].Hunks = combined
		} else {
			result = append(result, f)
			seen[f.Path] = len(result) - 1
		}
	}

	return result
}

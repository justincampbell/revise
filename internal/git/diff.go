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

	branch, err := DefaultBranch()
	if err != nil {
		return nil, err
	}

	mergeBase, err := MergeBase(branch)
	if err != nil {
		// If merge-base fails (e.g. detached HEAD with no common ancestor),
		// fall back to just showing uncommitted changes
		return getWorkingTreeDiff()
	}

	// Check if we're on the default branch (merge-base == HEAD)
	head, err := CurrentRef()
	if err != nil {
		return nil, err
	}

	if mergeBase == head {
		// On the default branch — show working tree changes
		return getWorkingTreeDiff()
	}

	// On a feature branch — show branch diff + working tree changes
	raw, err := RawDiff(mergeBase)
	if err != nil {
		return nil, err
	}

	diff := Parse(raw)

	// Merge in working tree changes (staged + unstaged)
	wtDiff, err := getWorkingTreeDiff()
	if err != nil {
		return nil, err
	}
	diff.Files = mergeFileDiffs(diff.Files, wtDiff.Files)

	return diff, nil
}

// getWorkingTreeDiff returns staged + unstaged + untracked changes.
func getWorkingTreeDiff() (*Diff, error) {
	staged, err := StagedDiff()
	if err != nil {
		return nil, err
	}
	unstaged, err := UnstagedDiff()
	if err != nil {
		return nil, err
	}

	diff := Parse(staged + unstaged)

	untracked, err := UntrackedFiles()
	if err != nil {
		return nil, err
	}
	diff.Files = append(diff.Files, untracked...)

	return diff, nil
}

// mergeFileDiffs combines branch diffs with working tree diffs.
// Working tree entries for the same path replace branch entries
// (they represent the latest state). New paths are appended.
func mergeFileDiffs(branch, wt []FileDiff) []FileDiff {
	seen := make(map[string]int, len(branch))
	for i, f := range branch {
		seen[f.Path] = i
	}

	for _, f := range wt {
		if idx, ok := seen[f.Path]; ok {
			// Working tree has newer changes for this file — replace
			branch[idx] = f
		} else {
			branch = append(branch, f)
		}
	}

	return branch
}

package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RemoteName returns the name of the tracking remote.
// It prefers "origin" if present, otherwise returns the first configured remote.
// Returns an empty string if there are no remotes.
func RemoteName() string {
	out, err := exec.Command("git", "remote").Output()
	if err != nil {
		return ""
	}
	remotes := strings.Fields(strings.TrimSpace(string(out)))
	for _, r := range remotes {
		if r == "origin" {
			return "origin"
		}
	}
	if len(remotes) > 0 {
		return remotes[0]
	}
	return ""
}

// DefaultBranch detects the default branch of the repository.
func DefaultBranch() (string, error) {
	remote := RemoteName()
	if remote != "" {
		// Try to get the default branch from the remote
		out, err := exec.Command("git", "rev-parse", "--verify", "--quiet", "refs/remotes/"+remote+"/main").Output()
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			return "main", nil
		}

		out, err = exec.Command("git", "rev-parse", "--verify", "--quiet", "refs/remotes/"+remote+"/master").Output()
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			return "master", nil
		}

		// Fallback: check remote HEAD
		out, err = exec.Command("git", "symbolic-ref", "refs/remotes/"+remote+"/HEAD").Output()
		if err == nil {
			ref := strings.TrimSpace(string(out))
			// refs/remotes/origin/main -> main
			parts := strings.Split(ref, "/")
			if len(parts) > 0 {
				return parts[len(parts)-1], nil
			}
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

// StatusFingerprint returns a string that changes whenever the working tree
// or index changes. It combines `git status --porcelain` (file-level status
// including untracked) with mtimes of dirty working tree files for
// content-level sensitivity.
func StatusFingerprint() (string, error) {
	status, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return "", fmt.Errorf("git status: %w", err)
	}

	var sb strings.Builder
	sb.Write(status)

	// Include mtimes of dirty working tree files so that content edits
	// to already-modified files are detected.
	for _, line := range strings.Split(strings.TrimRight(string(status), "\n"), "\n") {
		if len(line) < 4 {
			continue
		}
		path := line[3:]
		if info, err := os.Stat(path); err == nil {
			fmt.Fprintf(&sb, "\n%s:%d", path, info.ModTime().UnixNano())
		}
	}

	return sb.String(), nil
}

// DefaultContextLines is the default number of context lines in unified diffs.
const DefaultContextLines = 3

// RawDiff returns the raw unified diff from the merge-base to HEAD.
func RawDiff(mergeBase string, contextLines int) (string, error) {
	return RawDiffBetween(mergeBase, "HEAD", contextLines)
}

// RawDiffBetween returns the raw unified diff between two refs.
func RawDiffBetween(from, to string, contextLines int) (string, error) {
	args := []string{"diff", fmt.Sprintf("-U%d", contextLines), from, to}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git diff failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

// RawDiffBetweenIgnoreWhitespace returns the raw unified diff between two refs, ignoring whitespace.
func RawDiffBetweenIgnoreWhitespace(from, to string, contextLines int) (string, error) {
	args := []string{"diff", "-w", fmt.Sprintf("-U%d", contextLines), from, to}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git diff failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

// StagedDiff returns the raw diff of staged changes.
func StagedDiff(contextLines int) (string, error) {
	args := []string{"diff", fmt.Sprintf("-U%d", contextLines), "--cached"}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("git diff --cached: %w", err)
	}
	return string(out), nil
}

// StagedDiffIgnoreWhitespace returns the raw diff of staged changes, ignoring whitespace.
func StagedDiffIgnoreWhitespace(contextLines int) (string, error) {
	args := []string{"diff", "-w", fmt.Sprintf("-U%d", contextLines), "--cached"}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("git diff --cached: %w", err)
	}
	return string(out), nil
}

// UnstagedDiff returns the raw diff of unstaged changes.
func UnstagedDiff(contextLines int) (string, error) {
	args := []string{"diff", fmt.Sprintf("-U%d", contextLines)}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

// UnstagedDiffIgnoreWhitespace returns the raw diff of unstaged changes, ignoring whitespace.
func UnstagedDiffIgnoreWhitespace(contextLines int) (string, error) {
	args := []string{"diff", "-w", fmt.Sprintf("-U%d", contextLines)}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

// RepoRoot returns the absolute path to the top-level directory of the git repository.
func RepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
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

// resolveRef returns the commit hash for the given ref, or "" if it doesn't exist.
func resolveRef(ref string) string {
	out, err := exec.Command("git", "rev-parse", "--verify", "--quiet", ref).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// CurrentBranchName returns the name of the currently checked out branch,
// or "" if in detached HEAD state.
func CurrentBranchName() string {
	out, err := exec.Command("git", "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// IsOnDefaultBranch returns true if the current HEAD is at the merge-base
// with the default branch AND up to date with the remote tracking branch.
// Returns false when on a feature branch (merge-base != HEAD) or when on
// the default branch but the remote has diverged (useful for reviewing
// incoming/outgoing changes via branch mode).
func IsOnDefaultBranch() (bool, error) {
	branch, err := DefaultBranch()
	if err != nil {
		return false, err
	}

	remote := RemoteName()

	var mergeBase string
	if remote != "" {
		mergeBase, err = MergeBase(remote + "/" + branch)
	}
	if err != nil || mergeBase == "" {
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

	if mergeBase != head {
		return false, nil // feature branch with commits
	}

	// merge-base == HEAD: no commits diverging from the default branch.
	// If we're on a differently-named branch (feature branch with zero
	// commits), treat as default branch — show working tree only.
	currentBranch := CurrentBranchName()
	if currentBranch != "" && currentBranch != branch {
		return true, nil // feature branch with no commits — show working tree
	}

	// Actually on the default branch.
	// Check if the remote tracking branch has different commits.
	if remote != "" {
		remoteRef := resolveRef(remote + "/" + branch)
		if remoteRef != "" && remoteRef != head {
			return false, nil // default branch but not up to date with remote
		}
	}

	return true, nil
}

// resolveMergeBase finds the merge-base for the current branch.
func resolveMergeBase() (string, error) {
	branch, err := DefaultBranch()
	if err != nil {
		return "", err
	}

	var mergeBase string
	if remote := RemoteName(); remote != "" {
		mergeBase, err = MergeBase(remote + "/" + branch)
	}
	if err != nil || mergeBase == "" {
		mergeBase, err = MergeBase(branch)
	}
	if err != nil {
		return "", fmt.Errorf("no merge-base found: %w", err)
	}
	return mergeBase, nil
}

// BranchDiffOptions returns BranchDiff with optional whitespace ignoring.
func BranchDiffOptions(contextLines int, hideWhitespace bool) (*Diff, error) {
	if !hideWhitespace {
		return BranchDiff(contextLines)
	}

	mergeBase, err := resolveMergeBase()
	if err != nil {
		return nil, err
	}

	head, err := CurrentRef()
	if err != nil {
		return nil, err
	}

	var raw string
	if mergeBase == head {
		branch, err := DefaultBranch()
		if err != nil {
			return nil, err
		}
		remote := RemoteName()
		if remote == "" {
			return nil, fmt.Errorf("no remote configured")
		}
		raw, err = RawDiffBetweenIgnoreWhitespace(head, remote+"/"+branch, contextLines)
		if err != nil {
			return nil, err
		}
	} else {
		raw, err = RawDiffBetweenIgnoreWhitespace(mergeBase, "HEAD", contextLines)
		if err != nil {
			return nil, err
		}
	}

	diff := Parse(raw)
	tagHunks(diff.Files, SourceBranch)

	wtDiff, err := WorkingTreeDiffOptions(contextLines, true)
	if err != nil {
		return nil, err
	}
	diff.Files = mergeFileDiffs(diff.Files, wtDiff.Files)
	return diff, nil
}

// BranchDiff returns the merge-base diff merged with all working tree changes.
// This is the broadest view — committed + staged + unstaged + untracked.
// On the default branch behind the remote, it shows the remote's changes.
func BranchDiff(contextLines int) (*Diff, error) {
	mergeBase, err := resolveMergeBase()
	if err != nil {
		return nil, err
	}

	head, err := CurrentRef()
	if err != nil {
		return nil, err
	}

	var raw string
	if mergeBase == head {
		// merge-base == HEAD: we're on the default branch but the remote
		// has different commits. Diff HEAD against the remote ref.
		branch, err := DefaultBranch()
		if err != nil {
			return nil, err
		}
		remote := RemoteName()
		if remote == "" {
			return nil, fmt.Errorf("no remote configured")
		}
		raw, err = RawDiffBetween(head, remote+"/"+branch, contextLines)
		if err != nil {
			return nil, err
		}
	} else {
		raw, err = RawDiff(mergeBase, contextLines)
		if err != nil {
			return nil, err
		}
	}

	diff := Parse(raw)
	tagHunks(diff.Files, SourceBranch)

	wtDiff, err := WorkingTreeDiff(contextLines)
	if err != nil {
		return nil, err
	}
	// WorkingTreeDiff already tags its hunks as Staged/Unstaged
	diff.Files = mergeFileDiffs(diff.Files, wtDiff.Files)
	return diff, nil
}

// StagedOnlyDiffOptions returns only staged changes, optionally ignoring whitespace.
func StagedOnlyDiffOptions(contextLines int, hideWhitespace bool) (*Diff, error) {
	var raw string
	var err error
	if hideWhitespace {
		raw, err = StagedDiffIgnoreWhitespace(contextLines)
	} else {
		raw, err = StagedDiff(contextLines)
	}
	if err != nil {
		return nil, err
	}
	diff := Parse(raw)
	tagHunks(diff.Files, SourceStaged)
	return diff, nil
}

// StagedOnlyDiff returns only staged changes.
func StagedOnlyDiff(contextLines int) (*Diff, error) {
	raw, err := StagedDiff(contextLines)
	if err != nil {
		return nil, err
	}
	diff := Parse(raw)
	tagHunks(diff.Files, SourceStaged)
	return diff, nil
}

// UnstagedOnlyDiffOptions returns unstaged changes + untracked files, optionally ignoring whitespace.
func UnstagedOnlyDiffOptions(contextLines int, hideWhitespace bool) (*Diff, error) {
	var raw string
	var err error
	if hideWhitespace {
		raw, err = UnstagedDiffIgnoreWhitespace(contextLines)
	} else {
		raw, err = UnstagedDiff(contextLines)
	}
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

// UnstagedOnlyDiff returns unstaged changes + untracked files.
func UnstagedOnlyDiff(contextLines int) (*Diff, error) {
	raw, err := UnstagedDiff(contextLines)
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
func WorkingTreeDiff(contextLines int) (*Diff, error) {
	return getWorkingTreeDiff(contextLines, false)
}

// WorkingTreeDiffOptions returns staged + unstaged + untracked changes, optionally ignoring whitespace.
func WorkingTreeDiffOptions(contextLines int, hideWhitespace bool) (*Diff, error) {
	return getWorkingTreeDiff(contextLines, hideWhitespace)
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
		return WorkingTreeDiff(DefaultContextLines)
	}

	return BranchDiff(DefaultContextLines)
}

// getWorkingTreeDiff returns staged + unstaged + untracked changes.
func getWorkingTreeDiff(contextLines int, hideWhitespace bool) (*Diff, error) {
	var stagedRaw, unstagedRaw string
	var err error
	if hideWhitespace {
		stagedRaw, err = StagedDiffIgnoreWhitespace(contextLines)
		if err != nil {
			return nil, err
		}
		unstagedRaw, err = UnstagedDiffIgnoreWhitespace(contextLines)
		if err != nil {
			return nil, err
		}
	} else {
		stagedRaw, err = StagedDiff(contextLines)
		if err != nil {
			return nil, err
		}
		unstagedRaw, err = UnstagedDiff(contextLines)
		if err != nil {
			return nil, err
		}
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
// If a file was added in base and deleted in overlay, it is removed entirely
// (net zero change — e.g. file added on branch then deleted in working tree).
func mergeFileDiffs(base, overlay []FileDiff) []FileDiff {
	result := make([]FileDiff, len(base))
	copy(result, base)

	seen := make(map[string]int, len(result))
	baseStatus := make(map[string]FileStatus, len(result))
	for i, f := range result {
		seen[f.Path] = i
		baseStatus[f.Path] = f.Status
	}

	remove := make(map[int]bool)
	for _, f := range overlay {
		if idx, ok := seen[f.Path]; ok {
			if f.Status == StatusDeleted && baseStatus[f.Path] == StatusAdded {
				// Added in base, deleted in overlay — net zero
				remove[idx] = true
			} else {
				// Same file — combine hunks
				combined := make([]Hunk, len(result[idx].Hunks), len(result[idx].Hunks)+len(f.Hunks))
				copy(combined, result[idx].Hunks)
				combined = append(combined, f.Hunks...)
				result[idx].Hunks = combined
			}
		} else {
			result = append(result, f)
			seen[f.Path] = len(result) - 1
		}
	}

	if len(remove) > 0 {
		filtered := make([]FileDiff, 0, len(result)-len(remove))
		for i, f := range result {
			if !remove[i] {
				filtered = append(filtered, f)
			}
		}
		return filtered
	}

	return result
}

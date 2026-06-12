package git

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"
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

// StatusFingerprint returns a string that changes whenever the working tree,
// index, or HEAD changes. It combines `git status --porcelain` (file-level
// status including untracked) with mtimes of dirty working tree files for
// content-level sensitivity, plus the HEAD ref so commits register even when
// they leave the working tree in a state identical to the pre-stage one (e.g.
// rapid add+commit collapsing into a single poll window).
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

	// Include HEAD so a commit triggers a refresh even when the resulting
	// working tree fingerprint matches the pre-stage state. Without this,
	// IsOnDefaultBranch() can't re-evaluate and mode auto-promotion stalls.
	// resolveRef returns "" if HEAD is unborn (no commits yet) — that's fine.
	fmt.Fprintf(&sb, "\nHEAD:%s", resolveRef("HEAD"))

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

// GitDir returns the absolute path to the git directory for the current
// working tree. In a worktree this is `<main-repo>/.git/worktrees/<name>/`,
// not the worktree's `.git` file. Used by fswatch to monitor index/HEAD
// changes (per-worktree, not the main repo's).
func GitDir() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--absolute-git-dir").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --absolute-git-dir: %w", err)
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

// BranchDiff returns the merge-base diff merged with all working tree changes.
// This is the broadest view — committed + staged + unstaged + untracked.
// On the default branch behind the remote, it shows the remote's changes.
func BranchDiff(contextLines int) (*Diff, error) {
	diff, _, err := branchDiff(contextLines, false, 0)
	return diff, err
}

// BranchDiffOptions returns BranchDiff with optional whitespace ignoring.
func BranchDiffOptions(contextLines int, hideWhitespace bool) (*Diff, error) {
	diff, _, err := branchDiff(contextLines, hideWhitespace, 0)
	return diff, err
}

// BranchDiffDepth is like BranchDiffOptions but limits the committed range to
// the last `depth` commits (depth <= 0 means the full branch, from the
// merge-base). Working tree changes are always layered on top. It also returns
// the number of commits the branch is ahead of the merge-base, so callers can
// bound a depth selector — 0 when there are no local commits ahead (e.g. on the
// default branch behind the remote, where depth does not apply).
func BranchDiffDepth(contextLines int, hideWhitespace bool, depth int) (*Diff, int, error) {
	return branchDiff(contextLines, hideWhitespace, depth)
}

// branchDiff computes the committed range merged with working tree changes.
// The committed range runs from the merge-base to HEAD, optionally shortened to
// the last `depth` commits. The working tree diff runs concurrently with the
// branch diff computation. Returns the diff and the number of commits ahead of
// the merge-base.
func branchDiff(contextLines int, hideWhitespace bool, depth int) (*Diff, int, error) {
	// Start working tree diff immediately — it's independent of the branch diff.
	var wtDiff *Diff
	var g errgroup.Group
	g.Go(func() error {
		var err error
		wtDiff, err = WorkingTreeDiffOptions(contextLines, hideWhitespace)
		return err
	})

	// Meanwhile, compute branch diff.
	mergeBase, err := resolveMergeBase()
	if err != nil {
		_ = g.Wait()
		return nil, 0, err
	}

	head, err := CurrentRef()
	if err != nil {
		_ = g.Wait()
		return nil, 0, err
	}

	var raw string
	var commitsAhead int
	if mergeBase == head {
		// merge-base == HEAD: we're on the default branch but the remote
		// has different commits. Diff HEAD against the remote ref. There are
		// no local commits ahead, so depth does not apply (commitsAhead == 0).
		branch, err := DefaultBranch()
		if err != nil {
			_ = g.Wait()
			return nil, 0, err
		}
		remote := RemoteName()
		if remote == "" {
			_ = g.Wait()
			return nil, 0, fmt.Errorf("no remote configured")
		}
		raw, err = rawDiffBetween(head, remote+"/"+branch, contextLines, hideWhitespace)
		if err != nil {
			_ = g.Wait()
			return nil, 0, err
		}
	} else {
		commitsAhead = CommitsAhead(mergeBase)
		from := branchFromRef(mergeBase, depth, commitsAhead)
		raw, err = rawDiffBetween(from, "HEAD", contextLines, hideWhitespace)
		if err != nil {
			_ = g.Wait()
			return nil, 0, err
		}
	}

	diff := Parse(raw)

	if err := g.Wait(); err != nil {
		return nil, 0, err
	}

	diff.Files = composeBranch(diff.Files, wtDiff.Files)
	return diff, commitsAhead, nil
}

// rawDiffBetween dispatches to the whitespace-ignoring variant when requested.
func rawDiffBetween(from, to string, contextLines int, hideWhitespace bool) (string, error) {
	if hideWhitespace {
		return RawDiffBetweenIgnoreWhitespace(from, to, contextLines)
	}
	return RawDiffBetween(from, to, contextLines)
}

// branchFromRef returns the ref to diff from for a branch view at the given
// depth. depth <= 0 or depth >= commitsAhead yields the merge-base (the full
// branch); otherwise HEAD~depth, which is guaranteed to be after the merge-base.
// Falls back to the merge-base if HEAD~depth cannot be resolved.
func branchFromRef(mergeBase string, depth, commitsAhead int) string {
	if depth <= 0 || depth >= commitsAhead {
		return mergeBase
	}
	if ref := resolveRef(fmt.Sprintf("HEAD~%d", depth)); ref != "" {
		return ref
	}
	return mergeBase
}

// BranchCommits returns the commits the current branch is ahead of its
// merge-base with the default branch, newest first. Returns nil on the default
// branch, with no commits ahead, or when the merge-base can't be resolved.
// Used both to bound the Branch-mode depth selector and to render the commit
// list, without recomputing the whole diff.
func BranchCommits() ([]CommitInfo, error) {
	mergeBase, err := resolveMergeBase()
	if err != nil {
		return nil, nil // no merge-base — nothing to list
	}
	// %x1f is the ASCII unit separator: a delimiter that can't appear in a
	// single-line subject (%s), so fields parse unambiguously.
	out, err := exec.Command("git", "log", "--format=%H%x1f%h%x1f%s", mergeBase+"..HEAD").Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	var commits []CommitInfo
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 3)
		if len(parts) != 3 {
			continue
		}
		commits = append(commits, CommitInfo{SHA: parts[0], ShortSHA: parts[1], Subject: parts[2]})
	}
	return commits, nil
}

// CommitsAhead returns the number of commits reachable from HEAD but not from
// base (i.e. `git rev-list --count base..HEAD`). Returns 0 on error.
func CommitsAhead(base string) int {
	out, err := exec.Command("git", "rev-list", "--count", base+"..HEAD").Output()
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}
	return n
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
// In a repo with no commits, IsOnDefaultBranch returns true and the working
// tree diff still surfaces staged + untracked files via the empty-tree implicit
// base used by `git diff --cached`.
func GetDiff() (*Diff, error) {
	if !IsGitRepo() {
		return nil, fmt.Errorf("not a git repository")
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
// The three git operations run concurrently for faster results.
func getWorkingTreeDiff(contextLines int, hideWhitespace bool) (*Diff, error) {
	var stagedRaw, unstagedRaw string
	var untracked []FileDiff

	var g errgroup.Group
	g.Go(func() error {
		var err error
		if hideWhitespace {
			stagedRaw, err = StagedDiffIgnoreWhitespace(contextLines)
		} else {
			stagedRaw, err = StagedDiff(contextLines)
		}
		return err
	})
	g.Go(func() error {
		var err error
		if hideWhitespace {
			unstagedRaw, err = UnstagedDiffIgnoreWhitespace(contextLines)
		} else {
			unstagedRaw, err = UnstagedDiff(contextLines)
		}
		return err
	})
	g.Go(func() error {
		var err error
		untracked, err = UntrackedFiles()
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	staged := Parse(stagedRaw)
	unstaged := Parse(unstagedRaw)

	return &Diff{Files: composeWorkingTree(staged.Files, unstaged.Files, untracked)}, nil
}

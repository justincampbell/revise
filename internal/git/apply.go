package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// HunkPatch reconstructs a unified diff patch for a single hunk,
// suitable for piping to `git apply`.
func HunkPatch(path string, status FileStatus, h Hunk) string {
	var b strings.Builder

	fmt.Fprintf(&b, "diff --git a/%s b/%s\n", path, path)

	switch status {
	case StatusAdded:
		b.WriteString("--- /dev/null\n")
		fmt.Fprintf(&b, "+++ b/%s\n", path)
	case StatusDeleted:
		fmt.Fprintf(&b, "--- a/%s\n", path)
		b.WriteString("+++ /dev/null\n")
	default:
		fmt.Fprintf(&b, "--- a/%s\n", path)
		fmt.Fprintf(&b, "+++ b/%s\n", path)
	}

	fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", h.OldStart, h.OldCount, h.NewStart, h.NewCount)

	for _, line := range h.Lines {
		switch line.Type {
		case LineAdded:
			b.WriteString("+" + line.Content + "\n")
		case LineRemoved:
			b.WriteString("-" + line.Content + "\n")
		case LineContext:
			b.WriteString(" " + line.Content + "\n")
		}
	}

	return b.String()
}

// StageHunk stages a single hunk by applying the patch to the index.
// Untracked files don't exist in the index, so git apply --cached fails;
// fall back to git add for those.
func StageHunk(path string, status FileStatus, h Hunk) error {
	if status == StatusUntracked {
		return StageFile(path)
	}
	patch := HunkPatch(path, status, h)
	return gitApply(patch, "--cached")
}

// UnstageHunk unstages a single hunk by reverse-applying the patch from the index.
func UnstageHunk(path string, status FileStatus, h Hunk) error {
	patch := HunkPatch(path, status, h)
	return gitApply(patch, "-R", "--cached")
}

// DiscardHunk discards a hunk by reverting it from the working tree.
// For staged hunks, it first unstages, then reverts the working tree.
func DiscardHunk(path string, status FileStatus, h Hunk) error {
	patch := HunkPatch(path, status, h)
	if h.Source == SourceStaged {
		if err := gitApply(patch, "-R", "--cached"); err != nil {
			return err
		}
	}
	return gitApply(patch, "-R")
}

// StageFile stages an entire file.
func StageFile(path string) error {
	return withRetry(func() error {
		out, err := exec.Command("git", "add", "--", path).CombinedOutput()
		if err != nil {
			return fmt.Errorf("git add: %s\n%s", err, string(out))
		}
		return nil
	})
}

// UnstageFile unstages an entire file (keeps working tree changes).
func UnstageFile(path string) error {
	return withRetry(func() error {
		out, err := exec.Command("git", "reset", "HEAD", "--", path).CombinedOutput()
		if err != nil {
			return fmt.Errorf("git reset: %s\n%s", err, string(out))
		}
		return nil
	})
}

// DiscardFile discards all changes to a file.
// For untracked files, removes the file.
// If staged is true, unstages first, then reverts the working tree.
func DiscardFile(path string, status FileStatus, staged bool) error {
	if status == StatusUntracked {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove %s: %w", path, err)
		}
		return nil
	}
	if staged {
		if err := UnstageFile(path); err != nil {
			return err
		}
	}
	return withRetry(func() error {
		out, err := exec.Command("git", "checkout", "--", path).CombinedOutput()
		if err != nil {
			return fmt.Errorf("git checkout: %s\n%s", err, string(out))
		}
		return nil
	})
}

// isLockError returns true if the error is caused by a git index.lock contention.
func isLockError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "index.lock") || strings.Contains(msg, "unable to create") && strings.Contains(msg, "lock")
}

// withRetry retries fn up to 5 times with backoff when it fails due to lock contention.
func withRetry(fn func() error) error {
	const maxRetries = 5
	delay := 50 * time.Millisecond
	var err error
	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil || !isLockError(err) {
			return err
		}
		time.Sleep(delay)
		delay *= 2
	}
	return err
}

// gitApply runs git apply with the given flags, piping patch to stdin.
func gitApply(patch string, flags ...string) error {
	return withRetry(func() error {
		args := append([]string{"apply"}, flags...)
		cmd := exec.Command("git", args...)
		cmd.Stdin = strings.NewReader(patch)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git apply: %s\n%s", err, string(out))
		}
		return nil
	})
}

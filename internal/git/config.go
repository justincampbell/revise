package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// UntrackedCacheEnabled reports whether git's core.untrackedCache is enabled
// for the current repo. A missing config key is treated as not enabled.
// The untracked cache speeds up `git status` by caching directory mtimes —
// enabling it makes revise's polling loop noticeably faster on large repos.
func UntrackedCacheEnabled() bool {
	out, err := exec.Command("git", "config", "--get", "core.untrackedCache").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

// TestUntrackedCacheSupport runs git's built-in filesystem self-test.
// Returns nil if the filesystem reliably updates directory mtimes (required
// for the untracked cache), or an error otherwise. The test has small,
// transient filesystem side effects and should be treated as a one-shot check.
func TestUntrackedCacheSupport() error {
	cmd := exec.Command("git", "update-index", "--test-untracked-cache")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("filesystem does not support the untracked cache: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// EnableUntrackedCache sets core.untrackedCache=true in the current repo's
// local config. Local scope, not --global — we never modify the user's global
// git config implicitly.
func EnableUntrackedCache() error {
	cmd := exec.Command("git", "config", "core.untrackedCache", "true")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git config core.untrackedCache: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

package fswatch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCoalesce is the coalesce window used in tests — short so tests run fast.
const testCoalesce = 20 * time.Millisecond

// gitInit creates a fresh repo at dir, configures it for commits, and
// returns the resolved gitDir (worktree-aware via rev-parse).
func gitInit(t *testing.T) (workTree, gitDir string) {
	t.Helper()
	workTree = t.TempDir()
	for _, args := range [][]string{
		{"init"},
		{"symbolic-ref", "HEAD", "refs/heads/main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = workTree
		require.NoError(t, cmd.Run(), "git %s", strings.Join(args, " "))
	}

	out, err := runGit(workTree, "rev-parse", "--git-dir")
	require.NoError(t, err)
	gitDir = strings.TrimSpace(out)
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(workTree, gitDir)
	}
	return workTree, gitDir
}

func runGit(workTree string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = workTree
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func commitFile(t *testing.T, workTree, relPath, content string) {
	t.Helper()
	writeFile(t, filepath.Join(workTree, relPath), content)
	_, err := runGit(workTree, "add", relPath)
	require.NoError(t, err)
	_, err = runGit(workTree, "commit", "-m", "add "+relPath)
	require.NoError(t, err)
}

// expectEvent waits up to timeout for an Event on w.Events(), failing
// the test if none arrives.
func expectEvent(t *testing.T, w *Watcher, timeout time.Duration) {
	t.Helper()
	select {
	case <-w.Events():
	case <-time.After(timeout):
		t.Fatal("expected fswatch Event within", timeout)
	}
}

// expectNoEvent ensures no Event arrives within timeout.
func expectNoEvent(t *testing.T, w *Watcher, timeout time.Duration) {
	t.Helper()
	select {
	case <-w.Events():
		t.Fatal("expected no fswatch Event, but one arrived")
	case <-time.After(timeout):
	}
}

func TestWatcher_FiresOnTrackedFileEdit(t *testing.T) {
	workTree, gitDir := gitInit(t)
	commitFile(t, workTree, "foo.go", "package main\n")

	w, err := New(workTree, gitDir, testCoalesce)
	require.NoError(t, err)
	defer w.Close() //nolint:errcheck

	writeFile(t, filepath.Join(workTree, "foo.go"), "package main\n// edited\n")
	expectEvent(t, w, 1*time.Second)
}

func TestWatcher_FiresOnNewFileInTrackedDir(t *testing.T) {
	workTree, gitDir := gitInit(t)
	commitFile(t, workTree, "pkg/a.go", "package pkg\n")

	w, err := New(workTree, gitDir, testCoalesce)
	require.NoError(t, err)
	defer w.Close() //nolint:errcheck

	writeFile(t, filepath.Join(workTree, "pkg", "b.go"), "package pkg\n")
	expectEvent(t, w, 1*time.Second)
}

func TestWatcher_FiresOnIndexChange(t *testing.T) {
	workTree, gitDir := gitInit(t)
	commitFile(t, workTree, "foo.go", "package main\n")

	w, err := New(workTree, gitDir, testCoalesce)
	require.NoError(t, err)
	defer w.Close() //nolint:errcheck

	// Drain any startup-induced events.
	drainEvents(w, 50*time.Millisecond)

	writeFile(t, filepath.Join(workTree, "bar.go"), "package main\n")
	_, err = runGit(workTree, "add", "bar.go")
	require.NoError(t, err)

	expectEvent(t, w, 1*time.Second)
}

func TestWatcher_IgnoresUnrelatedGitDirFiles(t *testing.T) {
	workTree, gitDir := gitInit(t)
	commitFile(t, workTree, "foo.go", "package main\n")

	w, err := New(workTree, gitDir, testCoalesce)
	require.NoError(t, err)
	defer w.Close() //nolint:errcheck

	drainEvents(w, 50*time.Millisecond)

	// Touch an unrelated file inside .git that shouldn't trigger refresh.
	writeFile(t, filepath.Join(gitDir, "COMMIT_EDITMSG"), "draft message\n")
	expectNoEvent(t, w, 200*time.Millisecond)
}

func TestWatcher_CoalescesBurst(t *testing.T) {
	workTree, gitDir := gitInit(t)
	commitFile(t, workTree, "foo.go", "package main\n")

	w, err := New(workTree, gitDir, 100*time.Millisecond)
	require.NoError(t, err)
	defer w.Close() //nolint:errcheck

	drainEvents(w, 50*time.Millisecond)

	// Many rapid writes within the coalesce window should collapse into 1 Event.
	for i := 0; i < 5; i++ {
		writeFile(t, filepath.Join(workTree, "foo.go"), "edit "+string(rune('a'+i))+"\n")
		time.Sleep(5 * time.Millisecond)
	}

	expectEvent(t, w, 1*time.Second)
	expectNoEvent(t, w, 200*time.Millisecond)
}

func TestWatcher_FiresOnFileInNewlyCreatedDir(t *testing.T) {
	workTree, gitDir := gitInit(t)
	commitFile(t, workTree, "foo.go", "package main\n")

	w, err := New(workTree, gitDir, testCoalesce)
	require.NoError(t, err)
	defer w.Close() //nolint:errcheck

	// `mkdir new-feature && touch new-feature/foo.go` — the dir didn't
	// exist when New ran, so it wasn't in the initial watch set. The
	// watcher must pick it up dynamically, otherwise the second event
	// (file write inside the new dir) is invisible.
	require.NoError(t, os.MkdirAll(filepath.Join(workTree, "new-feature"), 0o755))

	// Drain the CREATE-of-dir event itself.
	expectEvent(t, w, 1*time.Second)

	// Give the watcher a moment to install the dynamic watch on
	// new-feature/ before we write inside it.
	time.Sleep(50 * time.Millisecond)

	writeFile(t, filepath.Join(workTree, "new-feature", "foo.go"), "package newfeature\n")
	expectEvent(t, w, 1*time.Second)
}

func TestWatcher_CloseIsIdempotent(t *testing.T) {
	workTree, gitDir := gitInit(t)
	commitFile(t, workTree, "foo.go", "package main\n")

	w, err := New(workTree, gitDir, testCoalesce)
	require.NoError(t, err)

	assert.NoError(t, w.Close())
	assert.NoError(t, w.Close(), "second Close should be a no-op")
}

func TestWatcher_NoEventsAfterClose(t *testing.T) {
	workTree, gitDir := gitInit(t)
	commitFile(t, workTree, "foo.go", "package main\n")

	w, err := New(workTree, gitDir, testCoalesce)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	writeFile(t, filepath.Join(workTree, "foo.go"), "edited\n")

	// Events channel should be closed; reading yields zero value immediately.
	select {
	case _, ok := <-w.Events():
		assert.False(t, ok, "Events channel should be closed after Close")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Events channel was not closed after Close")
	}
}

func TestWatcher_RefusesAboveWatchCap(t *testing.T) {
	workTree, gitDir := gitInit(t)
	// Each commit lands in its own directory, producing 10 unique tracked dirs.
	for i := 0; i < 10; i++ {
		commitFile(t, workTree, fmt.Sprintf("d%d/f.go", i), "package x\n")
	}

	orig := MaxWatches
	MaxWatches = 5
	defer func() { MaxWatches = orig }()

	w, err := New(workTree, gitDir, testCoalesce)
	if w != nil {
		_ = w.Close()
	}
	require.Error(t, err, "New should refuse when tracked dirs exceed MaxWatches")
	assert.Contains(t, err.Error(), "watch cap")
}

func TestWatcher_RuntimeAddsRespectWatchCap(t *testing.T) {
	workTree, gitDir := gitInit(t)
	commitFile(t, workTree, "foo.go", "package main\n")

	// Set a tight cap so a single new directory pushes us over.
	// Initial watches: 1 gitDir + 1 workTree = 2; cap at 2 means
	// any new dir created at runtime must be refused.
	orig := MaxWatches
	MaxWatches = 2
	defer func() { MaxWatches = orig }()

	w, err := New(workTree, gitDir, testCoalesce)
	require.NoError(t, err)
	defer w.Close() //nolint:errcheck

	startCount := w.watchCount.Load()

	// Create a new directory inside the work tree. maybeWatchNewDir
	// should observe the cap and refuse to add the watch.
	require.NoError(t, os.MkdirAll(filepath.Join(workTree, "new-dir"), 0o755))
	// Give the watcher goroutine a moment to handle the create event.
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, startCount, w.watchCount.Load(), "watchCount must not grow past MaxWatches")
}

func TestWatcher_AcceptsAtOrBelowWatchCap(t *testing.T) {
	workTree, gitDir := gitInit(t)
	for i := 0; i < 3; i++ {
		commitFile(t, workTree, fmt.Sprintf("d%d/f.go", i), "package x\n")
	}

	orig := MaxWatches
	MaxWatches = 100
	defer func() { MaxWatches = orig }()

	w, err := New(workTree, gitDir, testCoalesce)
	require.NoError(t, err)
	require.NoError(t, w.Close())
}

// drainEvents reads any pending Events for d duration, discarding them.
// Used to clear startup noise before a test makes its own changes.
func drainEvents(w *Watcher, d time.Duration) {
	deadline := time.After(d)
	for {
		select {
		case <-w.Events():
		case <-deadline:
			return
		}
	}
}

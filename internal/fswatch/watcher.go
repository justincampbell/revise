// Package fswatch wakes the auto-refresh loop when working-tree files
// or the git index change. It wraps fsnotify with two narrow concerns:
// gitignore-correct directory selection (via `git ls-files`, so we
// don't add watches for node_modules etc.) and burst coalescing (so
// one editor save fires one Event, not four).
//
// The Watcher emits a content-free Event — a wakeup signal. The caller
// is responsible for re-running git status / loadDiff and deciding
// whether to surface a change. This keeps the watcher decoupled from
// any UI or git-comparison logic.
package fswatch

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// MaxWatches caps the number of fsnotify watches installed by New.
// Each watch holds an fd on macOS (kqueue) and Linux (inotify), so
// large repos × many concurrent revise instances can exhaust the
// system fd limit (kern.maxfiles on macOS). When the unique tracked-
// directory count would exceed this cap, New returns an error so the
// caller can fall back to timer-only refresh. Exported as a var so
// tests can override it.
var MaxWatches = 500

// Event is a wakeup signal — it carries no data. The caller re-checks
// state via whatever primitive is appropriate (e.g. git.StatusFingerprint).
type Event struct{}

// Watcher emits Events when files in the working tree or git index change.
// Use New to construct one and Events()/Close() to consume and shut down.
type Watcher struct {
	fsn      *fsnotify.Watcher
	workTree string
	gitDir   string
	coalesce time.Duration

	out chan Event

	closeOnce sync.Once
	done      chan struct{}
}

// New starts a watcher over the working tree and git directory.
//
// workTree should be the absolute repo root; gitDir should be the
// worktree-aware git directory (`git rev-parse --git-dir`). The watcher
// adds fsnotify watches on the gitDir itself plus the unique parent
// directories of files reported by `git ls-files`. Recursion is
// avoided — large unrelated subtrees (node_modules, build outputs)
// are never watched because git already filters them.
//
// coalesce is the burst window: rapid events within coalesce of each
// other collapse into a single emitted Event.
//
// On any error during setup, returns the error and the caller should
// fall back to timer-only polling.
func New(workTree, gitDir string, coalesce time.Duration) (*Watcher, error) {
	if workTree == "" || gitDir == "" {
		return nil, errors.New("fswatch: workTree and gitDir are required")
	}
	fsn, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		fsn:      fsn,
		workTree: workTree,
		gitDir:   gitDir,
		coalesce: coalesce,
		out:      make(chan Event, 1),
		done:     make(chan struct{}),
	}

	if err := fsn.Add(gitDir); err != nil {
		_ = fsn.Close()
		return nil, err
	}

	if err := w.addTrackedDirs(); err != nil {
		_ = fsn.Close()
		return nil, err
	}

	go w.run()
	return w, nil
}

// Events returns the receive-only channel of wakeup events. The
// channel is closed when the Watcher is Closed.
func (w *Watcher) Events() <-chan Event { return w.out }

// Close stops the watcher and releases its resources. Safe to call
// multiple times.
func (w *Watcher) Close() error {
	var err error
	w.closeOnce.Do(func() {
		close(w.done)
		err = w.fsn.Close()
	})
	return err
}

// addTrackedDirs adds fsnotify watches on the unique parent directories
// of files reported by `git ls-files`. The unique set is computed before
// any Add calls so we can refuse cleanly when the count would exceed
// MaxWatches — adding partial watches and then bailing would leak fds.
// Failures on individual Adds are non-fatal — a missing watch just means
// missed events for that dir, not a broken watcher.
func (w *Watcher) addTrackedDirs() error {
	cmd := exec.Command("git", "-C", w.workTree, "ls-files")
	out, err := cmd.Output()
	if err != nil {
		return err
	}

	seen := make(map[string]struct{})
	// Always watch the working tree root so top-level files are covered.
	seen[w.workTree] = struct{}{}

	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		dir := filepath.Join(w.workTree, filepath.Dir(line))
		seen[dir] = struct{}{}
	}

	if len(seen) > MaxWatches {
		return fmt.Errorf("fswatch: %d tracked dirs exceeds watch cap of %d", len(seen), MaxWatches)
	}

	for dir := range seen {
		_ = w.fsn.Add(dir)
	}
	return nil
}

// run is the watcher goroutine: it filters incoming fsnotify events,
// coalesces bursts within w.coalesce, and emits one Event per burst.
func (w *Watcher) run() {
	defer close(w.out)

	for {
		select {
		case <-w.done:
			return
		case ev, ok := <-w.fsn.Events:
			if !ok {
				return
			}
			w.maybeWatchNewDir(ev)
			if !w.relevant(ev) {
				continue
			}
			// Drain further events within the coalesce window before
			// emitting. This collapses editor-save bursts (vim's
			// open/write/chmod/rename sequence) into one Event.
			w.drainBurst()
			select {
			case w.out <- Event{}:
			default:
				// Channel buffer full — a previous Event hasn't been
				// consumed yet. Dropping is fine: the consumer's next
				// read still triggers a refresh that covers our event.
			}
		case <-w.fsn.Errors:
			// fsnotify error channel — we don't surface these. A real
			// failure (watcher closed) is observed via the Events
			// channel close above.
		}
	}
}

// drainBurst consumes events for w.coalesce duration without emitting,
// so a flurry of related events results in only one downstream Event.
// We still inspect each event for new-directory creates so a
// `mkdir -p a/b/c && touch a/b/c/foo` burst leaves all of a/, a/b/,
// a/b/c/ watched.
func (w *Watcher) drainBurst() {
	deadline := time.NewTimer(w.coalesce)
	defer deadline.Stop()
	for {
		select {
		case <-w.done:
			return
		case <-deadline.C:
			return
		case ev, ok := <-w.fsn.Events:
			if !ok {
				return
			}
			w.maybeWatchNewDir(ev)
		}
	}
}

// maybeWatchNewDir installs a watch on a newly-created directory so
// subsequent edits inside it fire events. Without this, the user's
// `mkdir new-feature && touch new-feature/foo.go` would be invisible
// to fsnotify until the next periodic poll picked it up — usually
// fine, but slow on a 30s-clamped policy.
//
// We don't filter against gitignore here; gitignored content (e.g. an
// errant `node_modules`) won't change `git status` output, so the
// fingerprint check stays stable and no diff reload happens. The cost
// is a few extra fsnotify watches.
func (w *Watcher) maybeWatchNewDir(ev fsnotify.Event) {
	if ev.Op&fsnotify.Create == 0 {
		return
	}
	info, err := os.Stat(ev.Name)
	if err != nil || !info.IsDir() {
		return
	}
	_ = w.fsn.Add(ev.Name)
}

// relevant reports whether an fsnotify event should trigger a refresh.
// Events under gitDir only count if they touch index or HEAD; lockfiles,
// commit message drafts, and ref churn are filtered out.
func (w *Watcher) relevant(ev fsnotify.Event) bool {
	if strings.HasPrefix(ev.Name, w.gitDir) {
		base := filepath.Base(ev.Name)
		switch base {
		case "index", "HEAD":
			return true
		default:
			return false
		}
	}
	return true
}

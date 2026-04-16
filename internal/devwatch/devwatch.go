// Package devwatch polls a binary file for changes and invokes a callback
// when it is replaced. Used to support the --dev flag, which auto-restarts
// the running TUI when a freshly-built binary is installed.
package devwatch

import (
	"context"
	"os"
	"time"
)

// DefaultInterval is how often Watcher checks the binary by default.
const DefaultInterval = 500 * time.Millisecond

// Watcher polls a single file for modifications.
type Watcher struct {
	Path     string
	Interval time.Duration
	OnChange func()
}

// New returns a Watcher for the given path. OnChange is invoked exactly
// once, the first time a change is detected after Run starts.
func New(path string, onChange func()) *Watcher {
	return &Watcher{
		Path:     path,
		Interval: DefaultInterval,
		OnChange: onChange,
	}
}

// Run blocks, polling for changes, until ctx is cancelled or a change is
// detected and OnChange has been called. If the file cannot be stat-ed at
// startup, Run returns immediately.
func (w *Watcher) Run(ctx context.Context) {
	initial, err := stat(w.Path)
	if err != nil {
		return
	}
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current, err := stat(w.Path)
			if err != nil {
				continue
			}
			if changed(initial, current) {
				w.OnChange()
				return
			}
		}
	}
}

type fileSig struct {
	modTime time.Time
	size    int64
}

func stat(path string) (fileSig, error) {
	info, err := os.Stat(path)
	if err != nil {
		return fileSig{}, err
	}
	return fileSig{modTime: info.ModTime(), size: info.Size()}, nil
}

func changed(a, b fileSig) bool {
	return !a.modTime.Equal(b.modTime) || a.size != b.size
}

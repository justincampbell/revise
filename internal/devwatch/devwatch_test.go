package devwatch

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatcherFiresOnModTimeChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bin")
	if err := os.WriteFile(path, []byte("v1"), 0o755); err != nil {
		t.Fatal(err)
	}

	var fired atomic.Bool
	w := New(path, func() { fired.Store(true) })
	w.Interval = 10 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go w.Run(ctx)

	// Give the watcher time to record initial state.
	time.Sleep(50 * time.Millisecond)

	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if fired.Load() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("watcher did not fire on mtime change")
}

func TestWatcherFiresOnReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bin")
	if err := os.WriteFile(path, []byte("v1"), 0o755); err != nil {
		t.Fatal(err)
	}

	var fired atomic.Bool
	w := New(path, func() { fired.Store(true) })
	w.Interval = 10 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go w.Run(ctx)

	time.Sleep(50 * time.Millisecond)

	// Simulate `mv newbin path` — write a different size to a sibling, then rename over.
	tmp := filepath.Join(dir, "bin.new")
	if err := os.WriteFile(tmp, []byte("v2-larger"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if fired.Load() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("watcher did not fire on replacement")
}

func TestWatcherDoesNotFireWithoutChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bin")
	if err := os.WriteFile(path, []byte("v1"), 0o755); err != nil {
		t.Fatal(err)
	}

	var fired atomic.Bool
	w := New(path, func() { fired.Store(true) })
	w.Interval = 10 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	w.Run(ctx)

	if fired.Load() {
		t.Fatal("watcher fired without any change")
	}
}

func TestWatcherStopsOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bin")
	if err := os.WriteFile(path, []byte("v1"), 0o755); err != nil {
		t.Fatal(err)
	}

	w := New(path, func() { t.Fatal("callback should not fire") })
	w.Interval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watcher did not stop after context cancel")
	}
}

func TestWatcherFiresOnlyOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bin")
	if err := os.WriteFile(path, []byte("v1"), 0o755); err != nil {
		t.Fatal(err)
	}

	var count atomic.Int32
	w := New(path, func() { count.Add(1) })
	w.Interval = 10 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	go w.Run(ctx)

	time.Sleep(30 * time.Millisecond)

	// Two distinct changes.
	for i := 0; i < 2; i++ {
		future := time.Now().Add(time.Duration(i+1) * time.Hour)
		if err := os.Chtimes(path, future, future); err != nil {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	<-ctx.Done()
	if got := count.Load(); got != 1 {
		t.Fatalf("callback fired %d times; want 1", got)
	}
}

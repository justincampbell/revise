package git

import (
	"errors"
	"testing"
	"time"
)

func TestIsLockError(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{
			name:   "index.lock error",
			output: "fatal: Unable to create '/path/to/repo/.git/index.lock': File exists.",
			want:   true,
		},
		{
			name:   "other error",
			output: "error: patch failed: foo.go:1",
			want:   false,
		},
		{
			name:   "empty output",
			output: "",
			want:   false,
		},
		{
			name:   "index.lock in middle of output",
			output: "Some prefix\nfatal: Unable to create '/repo/.git/index.lock': File exists.\nSome suffix",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLockError(tt.output); got != tt.want {
				t.Errorf("isLockError(%q) = %v, want %v", tt.output, got, tt.want)
			}
		})
	}
}

func TestRetryOnLock_succeedsFirstTry(t *testing.T) {
	calls := 0
	err := retryOnLock(func() (string, error) {
		calls++
		return "", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestRetryOnLock_retriesOnLockError(t *testing.T) {
	calls := 0
	err := retryOnLock(func() (string, error) {
		calls++
		if calls < 3 {
			return "fatal: Unable to create '/repo/.git/index.lock': File exists.", errors.New("exit status 128")
		}
		return "", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetryOnLock_doesNotRetryOtherErrors(t *testing.T) {
	calls := 0
	err := retryOnLock(func() (string, error) {
		calls++
		return "error: patch failed", errors.New("exit status 1")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("expected 1 call (no retry for non-lock errors), got %d", calls)
	}
}

func TestRetryOnLock_failsAfterMaxRetries(t *testing.T) {
	calls := 0
	err := retryOnLock(func() (string, error) {
		calls++
		return "fatal: Unable to create '/repo/.git/index.lock': File exists.", errors.New("exit status 128")
	})
	if err == nil {
		t.Fatal("expected error after max retries")
	}
	if calls != maxLockRetries+1 {
		t.Errorf("expected %d calls, got %d", maxLockRetries+1, calls)
	}
}

func TestRetryOnLock_delaysBetweenRetries(t *testing.T) {
	calls := 0
	start := time.Now()
	retryOnLock(func() (string, error) {
		calls++
		if calls <= 2 {
			return "fatal: Unable to create '/repo/.git/index.lock': File exists.", errors.New("exit status 128")
		}
		return "", nil
	})
	elapsed := time.Since(start)
	// 2 retries * 100ms = 200ms minimum
	if elapsed < 150*time.Millisecond {
		t.Errorf("expected at least ~200ms delay for 2 retries, got %v", elapsed)
	}
}

package git

import (
	"strings"
	"time"
)

const (
	maxLockRetries = 3
	lockRetryDelay = 100 * time.Millisecond
)

// isLockError returns true if the output indicates git's index.lock contention.
func isLockError(output string) bool {
	return strings.Contains(output, "index.lock")
}

// retryOnLock retries fn up to maxLockRetries times when the output indicates
// lock contention (index.lock). Non-lock errors are returned immediately.
func retryOnLock(fn func() (string, error)) error {
	var lastErr error
	for attempt := 0; attempt <= maxLockRetries; attempt++ {
		output, err := fn()
		if err == nil {
			return nil
		}
		if !isLockError(output) {
			return err
		}
		lastErr = err
		if attempt < maxLockRetries {
			time.Sleep(lockRetryDelay)
		}
	}
	return lastErr
}

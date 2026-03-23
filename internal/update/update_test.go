package update

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckForUpdate_DevVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// "dev" version should always find a newer release (if any releases exist).
	info, err := CheckForUpdate("dev", false)
	if err != nil {
		t.Skipf("skipping: could not reach GitHub: %v", err)
	}
	// If no releases exist yet, info will be nil — that's OK.
	if info == nil {
		t.Log("no releases found on GitHub")
		return
	}
	assert.True(t, info.IsNewer)
	assert.NotEmpty(t, info.LatestVersion)
}

func TestCheckForUpdate_FutureVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	info, err := CheckForUpdate("999.0.0", false)
	if err != nil {
		t.Skipf("skipping: could not reach GitHub: %v", err)
	}
	if info == nil {
		t.Log("no releases found on GitHub")
		return
	}
	require.NotNil(t, info)
	assert.False(t, info.IsNewer)
}

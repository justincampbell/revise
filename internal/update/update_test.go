package update

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsSemver(t *testing.T) {
	assert.True(t, IsSemver("1.0.0"))
	assert.True(t, IsSemver("v1.0.0"))
	assert.True(t, IsSemver("0.1.0"))
	assert.True(t, IsSemver("1.2.3-beta"))
	assert.False(t, IsSemver("dev"))
	assert.False(t, IsSemver("dev-abc1234"))
	assert.False(t, IsSemver(""))
	assert.False(t, IsSemver("latest"))
}

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

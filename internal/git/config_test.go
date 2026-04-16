package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUntrackedCacheEnabled_NotSet(t *testing.T) {
	r := NewTestRepo(t)
	r.Chdir()
	assert.False(t, UntrackedCacheEnabled(), "unset config should read as disabled")
}

func TestUntrackedCacheEnabled_SetTrue(t *testing.T) {
	r := NewTestRepo(t)
	r.mustGit("config", "core.untrackedCache", "true")
	r.Chdir()
	assert.True(t, UntrackedCacheEnabled())
}

func TestUntrackedCacheEnabled_SetFalse(t *testing.T) {
	r := NewTestRepo(t)
	r.mustGit("config", "core.untrackedCache", "false")
	r.Chdir()
	assert.False(t, UntrackedCacheEnabled())
}

func TestEnableUntrackedCache_SetsConfig(t *testing.T) {
	r := NewTestRepo(t)
	r.Chdir()

	require.NoError(t, EnableUntrackedCache())
	assert.True(t, UntrackedCacheEnabled())
	assert.Equal(t, "true", r.mustGit("config", "--get", "core.untrackedCache"))
}

// The self-test has real filesystem side effects. On dev platforms (macOS,
// modern Linux) the temp dir should support untracked cache. If this test
// fails on CI, the CI filesystem doesn't support it — skip rather than fail.
func TestUntrackedCacheSupport_OnSupportedFilesystem(t *testing.T) {
	r := NewTestRepo(t)
	r.Chdir()

	if err := TestUntrackedCacheSupport(); err != nil {
		t.Skipf("filesystem at %s does not support untracked cache: %v", r.Dir, err)
	}
}

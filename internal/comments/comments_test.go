package comments_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/justincampbell/revise/internal/comments"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilePathFor_Deterministic(t *testing.T) {
	p1 := comments.FilePathFor("/home/user/myproject", "main")
	p2 := comments.FilePathFor("/home/user/myproject", "main")
	assert.Equal(t, p1, p2)
}

func TestFilePathFor_DifferentBranch(t *testing.T) {
	p1 := comments.FilePathFor("/home/user/myproject", "main")
	p2 := comments.FilePathFor("/home/user/myproject", "feature-x")
	assert.NotEqual(t, p1, p2)
}

func TestFilePathFor_DifferentRepo(t *testing.T) {
	p1 := comments.FilePathFor("/home/user/myproject", "main")
	p2 := comments.FilePathFor("/home/user/other", "main")
	assert.NotEqual(t, p1, p2)
}

func TestFilePathFor_Location(t *testing.T) {
	p := comments.FilePathFor("/home/user/myproject", "main")
	assert.Equal(t, "/tmp/revise", filepath.Dir(p))
	assert.Equal(t, ".json", filepath.Ext(p))
}

func TestSocketPath_Location(t *testing.T) {
	p := comments.SocketPath("/home/user/myproject", "main")
	assert.Equal(t, "/tmp/revise", filepath.Dir(p))
	assert.Equal(t, ".sock", filepath.Ext(p))
}

func TestSocketPath_SameHashAsFilePath(t *testing.T) {
	fp := comments.FilePathFor("/home/user/myproject", "main")
	sp := comments.SocketPath("/home/user/myproject", "main")
	// Both should use the same hash prefix
	fpBase := filepath.Base(fp)
	spBase := filepath.Base(sp)
	assert.Equal(t, fpBase[:len(fpBase)-5], spBase[:len(spBase)-5]) // strip extensions
}

func TestRead_NonExistent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review.json")
	rf, err := comments.Read(path)
	require.NoError(t, err)
	assert.Nil(t, rf)
}

func TestReadWrite_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review.json")

	original := &comments.ReviewFile{
		Repo:      "/home/user/project",
		Branch:    "feature-x",
		Submitted: false,
		Comments: []comments.Comment{
			{
				ID:     "abc123",
				File:   "main.go",
				Line:   42,
				Side:   "new",
				Body:   "This needs error handling",
				Status: "open",
			},
		},
	}

	require.NoError(t, comments.Write(path, original))

	got, err := comments.Read(path)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, original.Repo, got.Repo)
	assert.Equal(t, original.Branch, got.Branch)
	assert.Equal(t, original.Submitted, got.Submitted)
	require.Len(t, got.Comments, 1)
	assert.Equal(t, original.Comments[0].ID, got.Comments[0].ID)
	assert.Equal(t, original.Comments[0].Body, got.Comments[0].Body)
	assert.Equal(t, original.Comments[0].Status, got.Comments[0].Status)
}

func TestWrite_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "review.json")
	rf := &comments.ReviewFile{Repo: "/project", Branch: "main"}
	require.NoError(t, comments.Write(path, rf))
	_, err := os.Stat(path)
	require.NoError(t, err)
}

func TestWrite_Atomic_NoTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review.json")

	rf1 := &comments.ReviewFile{Repo: "/project", Branch: "main", Submitted: false}
	require.NoError(t, comments.Write(path, rf1))

	rf2 := &comments.ReviewFile{Repo: "/project", Branch: "main", Submitted: true}
	require.NoError(t, comments.Write(path, rf2))

	got, err := comments.Read(path)
	require.NoError(t, err)
	assert.True(t, got.Submitted)

	// Temp file should not be left behind
	_, err = os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err))
}

func TestReadWrite_WithReplies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review.json")

	original := &comments.ReviewFile{
		Repo:   "/project",
		Branch: "main",
		Comments: []comments.Comment{
			{
				ID:     "xyz789",
				File:   "foo.go",
				Line:   10,
				Side:   "new",
				Body:   "Fix this",
				Status: "closed",
				Replies: []comments.Reply{
					{Body: "Fixed in commit abc", Timestamp: "2026-01-01T00:00:00Z"},
				},
			},
		},
	}

	require.NoError(t, comments.Write(path, original))
	got, err := comments.Read(path)
	require.NoError(t, err)
	require.Len(t, got.Comments[0].Replies, 1)
	assert.Equal(t, "Fixed in commit abc", got.Comments[0].Replies[0].Body)
	assert.Equal(t, "closed", got.Comments[0].Status)
}

func TestNewID_Length(t *testing.T) {
	id := comments.NewID()
	assert.Len(t, id, 6)
}

func TestNewID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		id := comments.NewID()
		assert.False(t, seen[id], "duplicate ID: %s", id)
		seen[id] = true
	}
}

func TestNewID_AlphanumericOnly(t *testing.T) {
	for range 20 {
		id := comments.NewID()
		for _, c := range id {
			assert.True(t, (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'),
				"unexpected char %c in ID %s", c, id)
		}
	}
}

func TestTimestamp_RFC3339(t *testing.T) {
	ts := comments.Timestamp()
	assert.NotEmpty(t, ts)
	// Should be parseable as RFC3339
	assert.Contains(t, ts, "T")
	assert.Contains(t, ts, "Z")
}

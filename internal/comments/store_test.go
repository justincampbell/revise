package comments

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorePath_DeterministicForSameInput(t *testing.T) {
	p1 := StorePath("/repo", "feature")
	p2 := StorePath("/repo", "feature")
	assert.Equal(t, p1, p2)
}

func TestStorePath_DiffersForDifferentRepo(t *testing.T) {
	p1 := StorePath("/repo-a", "main")
	p2 := StorePath("/repo-b", "main")
	assert.NotEqual(t, p1, p2)
}

func TestStorePath_DiffersForDifferentBranch(t *testing.T) {
	p1 := StorePath("/repo", "main")
	p2 := StorePath("/repo", "feature")
	assert.NotEqual(t, p1, p2)
}

func TestStorePath_IsInTmpDir(t *testing.T) {
	p := StorePath("/repo", "main")
	assert.Contains(t, p, "revise")
	assert.True(t, filepath.IsAbs(p))
}

func TestStorePath_HasJSONExtension(t *testing.T) {
	p := StorePath("/repo", "main")
	assert.Equal(t, ".json", filepath.Ext(p))
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "comments.json")

	entries := map[string]string{
		"file.go:10:false": "fix this",
		"main.go:5:true":   "why removed?",
	}

	err := Save(path, entries)
	require.NoError(t, err)

	loaded, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, entries, loaded)
}

func TestLoad_NonExistentFile_ReturnsEmptyMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doesnotexist.json")

	loaded, err := Load(path)
	require.NoError(t, err)
	assert.NotNil(t, loaded)
	assert.Empty(t, loaded)
}

func TestSave_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "comments.json")

	entries := map[string]string{"a:1:false": "hello"}
	err := Save(path, entries)
	require.NoError(t, err)

	loaded, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, entries, loaded)
}

func TestSave_EmptyMap_WritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "comments.json")

	err := Save(path, map[string]string{})
	require.NoError(t, err)

	loaded, err := Load(path)
	require.NoError(t, err)
	assert.Empty(t, loaded)
}

func TestLoad_CorruptedFile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "comments.json")
	err := os.WriteFile(path, []byte("not json"), 0644)
	require.NoError(t, err)

	_, err = Load(path)
	assert.Error(t, err)
}

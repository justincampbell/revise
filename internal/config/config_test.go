package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefault(t *testing.T) {
	d := Default()
	assert.Equal(t, "dark", d.Theme)
	assert.Equal(t, "branch", d.DefaultMode)
	assert.Equal(t, 3, d.ContextLines)
	assert.True(t, d.Whitespace)
	assert.True(t, d.Mouse)
	assert.Equal(t, "true", d.UpdateCheck)
}

func TestPath_XDGSet(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	assert.Equal(t, "/custom/config/revise/config.yaml", Path())
}

func TestPath_XDGUnset(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	p := Path()
	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, ".config", "revise", "config.yaml"), p)
}

func TestLoad_NoFile(t *testing.T) {
	cfg, warnings, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Nil(t, cfg.Theme)
	assert.Nil(t, cfg.ContextLines)
}

func TestLoad_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(""), 0o644))

	cfg, warnings, err := Load(path)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Nil(t, cfg.Theme)
}

func TestLoad_PartialFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("theme: light\n"), 0o644))

	cfg, warnings, err := Load(path)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	require.NotNil(t, cfg.Theme)
	assert.Equal(t, "light", *cfg.Theme)
	assert.Nil(t, cfg.ContextLines, "unset fields should be nil")
	assert.Nil(t, cfg.Mouse, "unset fields should be nil")
}

func TestLoad_FullFile(t *testing.T) {
	content := `theme: light
default_mode: staged_only
context_lines: 5
whitespace: false
mouse: false
update_check: dev
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, warnings, err := Load(path)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	require.NotNil(t, cfg.Theme)
	assert.Equal(t, "light", *cfg.Theme)
	require.NotNil(t, cfg.DefaultMode)
	assert.Equal(t, "staged_only", *cfg.DefaultMode)
	require.NotNil(t, cfg.ContextLines)
	assert.Equal(t, 5, *cfg.ContextLines)
	require.NotNil(t, cfg.Whitespace)
	assert.False(t, *cfg.Whitespace)
	require.NotNil(t, cfg.Mouse)
	assert.False(t, *cfg.Mouse)
	require.NotNil(t, cfg.UpdateCheck)
	assert.Equal(t, UpdateCheckDev, *cfg.UpdateCheck)
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("theme: :\nbad yaml["), 0o644))

	_, _, err := Load(path)
	assert.Error(t, err)
}

func TestLoad_UnknownKeys(t *testing.T) {
	content := `theme: dark
contxt_lines: 10
moose: true
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, warnings, err := Load(path)
	require.NoError(t, err)
	assert.Len(t, warnings, 2, "should warn about 2 unknown keys")
	// The known fields should still be parsed.
	require.NotNil(t, cfg.Theme)
	assert.Equal(t, "dark", *cfg.Theme)
}

func TestLoad_PermissionError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("theme: dark"), 0o000))

	_, _, err := Load(path)
	assert.Error(t, err)
}

func TestLoad_BOM(t *testing.T) {
	content := append([]byte{0xEF, 0xBB, 0xBF}, []byte("theme: light\n")...)
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, content, 0o644))

	cfg, _, err := Load(path)
	require.NoError(t, err)
	require.NotNil(t, cfg.Theme)
	assert.Equal(t, "light", *cfg.Theme)
}

func TestResolve_AllNil(t *testing.T) {
	r := Resolve(Config{})
	assert.Equal(t, Default(), r)
}

func TestResolve_PartialOverride(t *testing.T) {
	theme := "light"
	cfg := Config{Theme: &theme}
	r := Resolve(cfg)
	assert.Equal(t, "light", r.Theme)
	assert.Equal(t, 3, r.ContextLines, "unset fields should use defaults")
	assert.True(t, r.Mouse, "unset fields should use defaults")
}

func TestResolve_ContextLinesClamp(t *testing.T) {
	neg := -5
	r := Resolve(Config{ContextLines: &neg})
	assert.Equal(t, 0, r.ContextLines)

	big := 100
	r = Resolve(Config{ContextLines: &big})
	assert.Equal(t, 20, r.ContextLines)
}

func TestUpdateCheck_Bool(t *testing.T) {
	for _, tc := range []struct {
		input    string
		expected UpdateCheck
	}{
		{"update_check: true", UpdateCheckTrue},
		{"update_check: false", UpdateCheckFalse},
		{"update_check: yes", UpdateCheckTrue},
		{"update_check: no", UpdateCheckFalse},
	} {
		t.Run(tc.input, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tc.input), 0o644))
			cfg, _, err := Load(path)
			require.NoError(t, err)
			require.NotNil(t, cfg.UpdateCheck)
			assert.Equal(t, tc.expected, *cfg.UpdateCheck)
		})
	}
}

func TestUpdateCheck_Dev(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("update_check: dev"), 0o644))
	cfg, _, err := Load(path)
	require.NoError(t, err)
	require.NotNil(t, cfg.UpdateCheck)
	assert.Equal(t, UpdateCheckDev, *cfg.UpdateCheck)
}

func TestUpdateCheck_Invalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("update_check: banana"), 0o644))
	_, _, err := Load(path)
	assert.Error(t, err)
}

func TestGenerateTemplate_Roundtrip(t *testing.T) {
	// The template has all keys commented out, so loading it should
	// return all-nil fields (same as loading an empty file).
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(GenerateTemplate()), 0o644))

	cfg, warnings, err := Load(path)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Nil(t, cfg.Theme)
	assert.Nil(t, cfg.DefaultMode)
	assert.Nil(t, cfg.ContextLines)
	assert.Nil(t, cfg.Whitespace)
	assert.Nil(t, cfg.Mouse)
	assert.Nil(t, cfg.UpdateCheck)
}

func TestWriteTemplate_CreatesFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	path := filepath.Join(dir, "config.yaml")

	err := WriteTemplate(path, false)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, GenerateTemplate(), string(data))
}

func TestWriteTemplate_RefusesOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("existing"), 0o644))

	err := WriteTemplate(path, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestWriteTemplate_ForceOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("existing"), 0o644))

	err := WriteTemplate(path, true)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, GenerateTemplate(), string(data))
}

func TestExists_False(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	assert.False(t, Exists())
}

func TestExists_True(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "revise"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "revise", "config.yaml"), []byte(""), 0o644))
	assert.True(t, Exists())
}

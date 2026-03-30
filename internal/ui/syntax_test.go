package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeControlChars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"plain text", "hello", "hello"},
		{"tab preserved", "\t", "\t"},
		{"null byte", "\x00", "\u2400"},
		{"ESC", "\x1b", "\u241b"},
		{"DEL", "\x7f", "\u2421"},
		{"mixed", "a\x00b\tc", "a\u2400b\tc"},
		{"all control 0x01-0x1f except tab", "\x01\x09\x1f", "\u2401\t\u241f"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, escapeControlChars(tt.input))
		})
	}
}

func TestDetectIndentSize(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  int
	}{
		{"empty", []string{}, 0},
		{"no indentation", []string{"foo", "bar"}, 0},
		{"two spaces", []string{"  foo", "    bar"}, 2},
		{"four spaces", []string{"    foo"}, 4},
		{"tabs", []string{"\tfoo", "\t\tbar"}, 1},
		{"mixed — smallest wins", []string{"    foo", "  bar"}, 2},
		{"blank lines ignored", []string{"", "  foo"}, 2},
		{"single space", []string{" foo"}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, detectIndentSize(tt.lines))
		})
	}
}

func TestSplitLeadingWhitespace(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		indentSize int
		wantRest   string
	}{
		{"no indent", "foo", 2, "foo"},
		{"zero indentSize", "  foo", 0, "  foo"},
		{"two spaces one level", "  foo", 2, "foo"},
		{"four spaces two levels", "    foo", 2, "foo"},
		{"remainder spaces", "   foo", 2, "foo"},
		{"tabs", "\t\tfoo", 1, "foo"},
		{"empty string", "", 2, ""},
		{"only whitespace", "  ", 2, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, rest := splitLeadingWhitespace(tt.content, tt.indentSize, nil)
			assert.Equal(t, tt.wantRest, rest)
		})
	}

	t.Run("guides contain guide char for each level", func(t *testing.T) {
		guides, _ := splitLeadingWhitespace("    foo", 2, nil)
		count := strings.Count(guides, indentGuide)
		assert.Equal(t, 2, count)
	})

	t.Run("tab guides contain guide char for each tab", func(t *testing.T) {
		guides, _ := splitLeadingWhitespace("\t\tfoo", 1, nil)
		count := strings.Count(guides, indentGuide)
		assert.Equal(t, 2, count)
	})
}

func TestAddIndentGuides_NoColor(t *testing.T) {
	orig := noColor
	noColor = true
	defer func() { noColor = orig }()

	// noColor path should escape control chars but not add guides
	result := addIndentGuides("  \x1bfoo", 2, nil)
	assert.Equal(t, "  \u241bfoo", result)
	assert.NotContains(t, result, indentGuide)
}

func TestAddIndentGuides_WithColor(t *testing.T) {
	orig := noColor
	noColor = false
	defer func() { noColor = orig }()

	result := addIndentGuides("  foo", 2, nil)
	assert.Contains(t, result, indentGuide)
	assert.Contains(t, result, "foo")
}

func TestHighlightLine_CacheHit(t *testing.T) {
	orig := noColor
	noColor = false
	defer func() { noColor = orig }()

	clearHighlightCache()

	// First call — cache miss, should highlight a Go file
	result1, ok1 := highlightLine("package main", "main.go", nil, 0)
	assert.True(t, ok1)

	// Second call — should return cached result
	result2, ok2 := highlightLine("package main", "main.go", nil, 0)
	assert.True(t, ok2)
	assert.Equal(t, result1, result2)
}

func TestHighlightLine_NoColor(t *testing.T) {
	orig := noColor
	noColor = true
	defer func() { noColor = orig }()

	result, ok := highlightLine("package main", "main.go", nil, 0)
	assert.False(t, ok)
	assert.Equal(t, "package main", result)
}

func TestHighlightLine_UnknownExtension(t *testing.T) {
	orig := noColor
	noColor = false
	defer func() { noColor = orig }()

	result, ok := highlightLine("some content", "file.xyzunknown", nil, 0)
	assert.False(t, ok)
	assert.Equal(t, "some content", result)
}

func TestHighlightLine_CacheKeyIncludesTheme(t *testing.T) {
	orig := noColor
	noColor = false
	origTheme := activeTheme
	defer func() {
		noColor = orig
		activeTheme = origTheme
	}()

	clearHighlightCache()

	activeTheme = ThemeDark
	result1, _ := highlightLine("package main", "main.go", nil, 0)

	activeTheme = ThemeLight
	result2, _ := highlightLine("package main", "main.go", nil, 0)

	// Different themes should produce different cache entries (may differ in output)
	_ = result1
	_ = result2

	highlightCacheMu.Lock()
	count := len(highlightCache)
	highlightCacheMu.Unlock()
	assert.Equal(t, 2, count)
}

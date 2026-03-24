package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPadRight_ASCII(t *testing.T) {
	assert.Equal(t, "abc   ", padRight("abc", 6))
}

func TestPadRight_AlreadyLong(t *testing.T) {
	assert.Equal(t, "abcdefg", padRight("abcdefg", 4))
}

func TestPadRight_ExactLength(t *testing.T) {
	assert.Equal(t, "abcd", padRight("abcd", 4))
}

func TestPadRight_UnicodeArrow(t *testing.T) {
	// "← (h)" is 6 runes but more bytes; should pad to 14 runes
	got := padRight("← (h)", 14)
	assert.Equal(t, 14, len([]rune(got)))
}

func TestPluralize(t *testing.T) {
	assert.Equal(t, "0 comments", pluralize(0, "comment", "comments"))
	assert.Equal(t, "1 comment", pluralize(1, "comment", "comments"))
	assert.Equal(t, "2 comments", pluralize(2, "comment", "comments"))
	assert.Equal(t, "1 file", pluralize(1, "file", "files"))
	assert.Equal(t, "5 files", pluralize(5, "file", "files"))
}

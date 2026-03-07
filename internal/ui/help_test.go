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

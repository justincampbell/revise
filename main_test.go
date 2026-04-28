package main

import (
	"strings"
	"testing"
)

func TestLoadDiffForMode_UnknownMode(t *testing.T) {
	_, err := loadDiffForMode("bogus")
	if err == nil {
		t.Fatal("expected error for unknown mode")
	}
	msg := err.Error()
	if !strings.Contains(msg, "bogus") {
		t.Errorf("error should mention the bad mode value, got: %s", msg)
	}
	for _, valid := range []string{"branch", "staged", "staged-only", "unstaged"} {
		if !strings.Contains(msg, valid) {
			t.Errorf("error should list %q as a valid mode, got: %s", valid, msg)
		}
	}
}

// Package editor builds a command to open a file (and optionally a specific
// line) in the user's preferred editor, based on $VISUAL / $EDITOR.
package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DefaultEditor is used when neither $VISUAL nor $EDITOR is set.
const DefaultEditor = "vi"

// Command builds an *exec.Cmd that opens filePath in the user's editor at
// the given lineNum. Pass lineNum <= 0 to skip the line argument.
//
// $VISUAL is preferred over $EDITOR (the historical convention: $VISUAL is
// for full-screen editors, $EDITOR for line editors). Both env vars may
// include arguments (e.g. `code --wait`); they are tokenized on whitespace.
//
// The line-number argument format depends on the editor binary:
//   - vim/nvim/vi/nano/emacs/pico: `+N file`
//   - code/code-insiders/code-oss/cursor/codium/windsurf: `--goto file:N`
//   - subl/sublime_text/atom/zed/mate/rmate/hx (helix): `file:N`
//
// Whitespace splitting is naive (strings.Fields) — editor strings with
// quoted args or paths containing spaces aren't supported. Users with
// unusual setups can wrap their editor in a shell script.
func Command(filePath string, lineNum int) (*exec.Cmd, error) {
	ed := os.Getenv("VISUAL")
	if ed == "" {
		ed = os.Getenv("EDITOR")
	}
	if ed == "" {
		ed = DefaultEditor
	}

	parts := strings.Fields(ed)
	if len(parts) == 0 {
		return nil, fmt.Errorf("editor command is empty")
	}
	bin := parts[0]
	args := append([]string(nil), parts[1:]...)
	args = append(args, fileArgs(filepath.Base(bin), filePath, lineNum)...)
	return exec.Command(bin, args...), nil
}

// fileArgs returns the editor-specific arguments for opening filePath at
// lineNum. Pulled out for unit testing without invoking exec.
func fileArgs(bin, filePath string, lineNum int) []string {
	if lineNum <= 0 {
		return []string{filePath}
	}
	switch bin {
	case "code", "code-insiders", "code-oss", "codium", "vscodium", "cursor", "windsurf":
		return []string{"--goto", fmt.Sprintf("%s:%d", filePath, lineNum)}
	case "subl", "sublime_text", "atom", "zed", "mate", "rmate", "hx", "helix":
		return []string{fmt.Sprintf("%s:%d", filePath, lineNum)}
	default:
		// vi, vim, nvim, nano, emacs, emacsclient, pico, jove, joe…
		return []string{fmt.Sprintf("+%d", lineNum), filePath}
	}
}

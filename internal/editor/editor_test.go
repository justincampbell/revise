package editor

import (
	"reflect"
	"strings"
	"testing"
)

func TestFileArgs(t *testing.T) {
	tests := []struct {
		name    string
		bin     string
		path    string
		lineNum int
		want    []string
	}{
		{"vim with line", "vim", "main.go", 42, []string{"+42", "main.go"}},
		{"nvim with line", "nvim", "main.go", 42, []string{"+42", "main.go"}},
		{"vi with line", "vi", "main.go", 1, []string{"+1", "main.go"}},
		{"nano with line", "nano", "main.go", 7, []string{"+7", "main.go"}},
		{"emacs with line", "emacs", "main.go", 9, []string{"+9", "main.go"}},
		{"vscode --goto", "code", "main.go", 42, []string{"--goto", "main.go:42"}},
		{"vscode insiders", "code-insiders", "main.go", 1, []string{"--goto", "main.go:1"}},
		{"code-oss (linux)", "code-oss", "main.go", 1, []string{"--goto", "main.go:1"}},
		{"codium", "codium", "main.go", 1, []string{"--goto", "main.go:1"}},
		{"cursor", "cursor", "main.go", 1, []string{"--goto", "main.go:1"}},
		{"windsurf", "windsurf", "main.go", 1, []string{"--goto", "main.go:1"}},

		{"sublime", "subl", "main.go", 42, []string{"main.go:42"}},
		{"zed", "zed", "main.go", 5, []string{"main.go:5"}},
		{"atom", "atom", "main.go", 5, []string{"main.go:5"}},
		{"helix (hx)", "hx", "main.go", 5, []string{"main.go:5"}},
		{"helix (full name)", "helix", "main.go", 5, []string{"main.go:5"}},

		{"no line: skips line arg", "vim", "main.go", 0, []string{"main.go"}},
		{"negative line: skips line arg", "vim", "main.go", -1, []string{"main.go"}},
		{"unknown editor: defaults to +N FILE", "foobar", "main.go", 3, []string{"+3", "main.go"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileArgs(tt.bin, tt.path, tt.lineNum)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fileArgs(%q, %q, %d) = %v, want %v", tt.bin, tt.path, tt.lineNum, got, tt.want)
			}
		})
	}
}

func TestCommand_PicksVisualOverEditor(t *testing.T) {
	t.Setenv("VISUAL", "code --wait")
	t.Setenv("EDITOR", "vim")

	cmd, err := Command("main.go", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cmd.Path; !strings.HasSuffix(got, "code") {
		t.Errorf("Path = %q, want suffix 'code'", got)
	}
	// Args[0] is the binary; subsequent args are from the editor string and fileArgs.
	want := []string{"code", "--wait", "--goto", "main.go:10"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("Args = %v, want %v", cmd.Args, want)
	}
}

func TestCommand_FallsBackToEditor(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "vim")

	cmd, err := Command("main.go", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"vim", "+3", "main.go"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("Args = %v, want %v", cmd.Args, want)
	}
}

func TestCommand_DefaultsToVi(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")

	cmd, err := Command("main.go", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"vi", "main.go"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("Args = %v, want %v", cmd.Args, want)
	}
}

func TestCommand_EditorWithFlags(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "nvim -u NONE")

	cmd, err := Command("main.go", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"nvim", "-u", "NONE", "+7", "main.go"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("Args = %v, want %v", cmd.Args, want)
	}
}

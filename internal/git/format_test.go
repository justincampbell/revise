package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormat_ModifiedFile(t *testing.T) {
	d := &Diff{
		Files: []FileDiff{{
			Path:   "hello.go",
			Status: StatusModified,
			Hunks: []Hunk{{
				OldStart: 1, OldCount: 3, NewStart: 1, NewCount: 4,
				Lines: []Line{
					{Type: LineContext, Content: "package main"},
					{Type: LineRemoved, Content: "// old"},
					{Type: LineAdded, Content: "// new"},
					{Type: LineAdded, Content: "// extra"},
					{Type: LineContext, Content: "func main() {}"},
				},
			}},
		}},
	}

	got := Format(d)
	assert.Contains(t, got, "diff --git a/hello.go b/hello.go")
	assert.Contains(t, got, "--- a/hello.go")
	assert.Contains(t, got, "+++ b/hello.go")
	assert.Contains(t, got, "@@ -1,3 +1,4 @@")
	assert.Contains(t, got, "-// old")
	assert.Contains(t, got, "+// new")
	assert.Contains(t, got, "+// extra")
	assert.Contains(t, got, " package main")
}

func TestFormat_AddedFile(t *testing.T) {
	d := &Diff{
		Files: []FileDiff{{
			Path:   "new.go",
			Status: StatusAdded,
			Hunks: []Hunk{{
				OldStart: 0, OldCount: 0, NewStart: 1, NewCount: 1,
				Lines: []Line{
					{Type: LineAdded, Content: "package new"},
				},
			}},
		}},
	}

	got := Format(d)
	assert.Contains(t, got, "new file mode 100644")
	assert.Contains(t, got, "--- /dev/null")
	assert.Contains(t, got, "+++ b/new.go")
}

func TestFormat_DeletedFile(t *testing.T) {
	d := &Diff{
		Files: []FileDiff{{
			Path:   "old.go",
			Status: StatusDeleted,
			Hunks: []Hunk{{
				OldStart: 1, OldCount: 1, NewStart: 0, NewCount: 0,
				Lines: []Line{
					{Type: LineRemoved, Content: "package old"},
				},
			}},
		}},
	}

	got := Format(d)
	assert.Contains(t, got, "deleted file mode 100644")
	assert.Contains(t, got, "--- a/old.go")
	assert.Contains(t, got, "+++ /dev/null")
}

func TestFormat_Empty(t *testing.T) {
	d := &Diff{}
	assert.Equal(t, "", Format(d))
}

func TestFormat_RenamedFile(t *testing.T) {
	d := &Diff{
		Files: []FileDiff{{
			Path:    "new_name.go",
			OldPath: "old_name.go",
			Status:  StatusRenamed,
		}},
	}

	got := Format(d)
	assert.Contains(t, got, "rename from old_name.go")
	assert.Contains(t, got, "rename to new_name.go")
}

func TestFormatGutter_Added(t *testing.T) {
	l := Line{Type: LineAdded, NewNum: 42}
	assert.Equal(t, "   42 ", FormatGutter(l))
}

func TestFormatGutter_Removed(t *testing.T) {
	l := Line{Type: LineRemoved, OldNum: 7}
	assert.Equal(t, "    7 ", FormatGutter(l))
}

func TestFormatGutter_Context(t *testing.T) {
	l := Line{Type: LineContext, OldNum: 3, NewNum: 5}
	assert.Equal(t, "    5 ", FormatGutter(l))
}

func TestHunkContextText_ExtractsTrailingContext(t *testing.T) {
	got := HunkContextText("@@ -10,6 +12,7 @@ func renderStatusBar() string {")
	assert.Equal(t, "func renderStatusBar() string {", got)
}

func TestHunkContextText_NoAtAt(t *testing.T) {
	got := HunkContextText("some random text")
	assert.Equal(t, "some random text", got)
}

func TestHunkContextText_EmptyContext(t *testing.T) {
	got := HunkContextText("@@ -1,1 +1,1 @@")
	assert.Equal(t, "", got)
}

func TestHunkSourceLabel(t *testing.T) {
	assert.Equal(t, "branch", HunkSourceLabel(SourceBranch))
	assert.Equal(t, "staged", HunkSourceLabel(SourceStaged))
	assert.Equal(t, "unstaged", HunkSourceLabel(SourceUnstaged))
	assert.Equal(t, "", HunkSourceLabel(""))
}

func TestHunkHeaderText(t *testing.T) {
	tests := []struct {
		name string
		hunk Hunk
		want string
	}{
		{"no source no context", Hunk{Header: "@@ -1,1 +1,1 @@"}, ""},
		{"source only", Hunk{Header: "@@ -1,1 +1,1 @@", Source: SourceStaged}, "[staged]"},
		{"context only", Hunk{Header: "@@ -1,1 +1,1 @@ funcName()"}, "funcName()"},
		{"source + context", Hunk{Header: "@@ -1,1 +1,1 @@ funcName()", Source: SourceBranch}, "[branch] funcName()"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HunkHeaderText(tt.hunk))
		})
	}
}

func TestFormatHunks_RendersFilePathAndGutter(t *testing.T) {
	d := &Diff{
		Files: []FileDiff{{
			Path:   "hello.go",
			Status: StatusModified,
			Hunks: []Hunk{{
				OldStart: 10, OldCount: 2, NewStart: 10, NewCount: 3,
				Header: "@@ -10,2 +10,3 @@ func main()",
				Source: SourceUnstaged,
				Lines: []Line{
					{Type: LineContext, Content: "x := 1", OldNum: 10, NewNum: 10},
					{Type: LineRemoved, Content: "old", OldNum: 11},
					{Type: LineAdded, Content: "new", NewNum: 11},
					{Type: LineAdded, Content: "extra", NewNum: 12},
				},
			}},
		}},
	}

	got := FormatHunks(d)
	assert.Contains(t, got, "hello.go\n")
	assert.Contains(t, got, "[unstaged] func main()")
	assert.Contains(t, got, "   10   x := 1")
	assert.Contains(t, got, "   11 - old")
	assert.Contains(t, got, "   11 + new")
	assert.Contains(t, got, "   12 + extra")
}

func TestFormatHunks_BinaryFile(t *testing.T) {
	d := &Diff{
		Files: []FileDiff{{
			Path:     "logo.png",
			Status:   StatusModified,
			IsBinary: true,
		}},
	}
	got := FormatHunks(d)
	assert.Contains(t, got, "logo.png")
	assert.Contains(t, got, "Binary file")
}

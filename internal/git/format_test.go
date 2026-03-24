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

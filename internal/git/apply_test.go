package git

import (
	"strings"
	"testing"
)

func TestHunkPatch(t *testing.T) {
	hunk := Hunk{
		OldStart: 1,
		OldCount: 3,
		NewStart: 1,
		NewCount: 4,
		Header:   "@@ -1,3 +1,4 @@ func main()",
		Lines: []Line{
			{Type: LineContext, Content: "package main", OldNum: 1, NewNum: 1},
			{Type: LineAdded, Content: "", NewNum: 2},
			{Type: LineContext, Content: "func base() {}", OldNum: 2, NewNum: 3},
			{Type: LineContext, Content: "", OldNum: 3, NewNum: 4},
		},
	}

	patch := HunkPatch("base.go", StatusModified, hunk)

	if !strings.HasPrefix(patch, "diff --git a/base.go b/base.go\n") {
		t.Errorf("expected diff header, got:\n%s", patch)
	}
	if !strings.Contains(patch, "--- a/base.go\n") {
		t.Errorf("expected --- a/base.go, got:\n%s", patch)
	}
	if !strings.Contains(patch, "+++ b/base.go\n") {
		t.Errorf("expected +++ b/base.go, got:\n%s", patch)
	}
	if !strings.Contains(patch, "@@ -1,3 +1,4 @@\n") {
		t.Errorf("expected hunk header, got:\n%s", patch)
	}
	if !strings.Contains(patch, "+\n") {
		t.Errorf("expected added line, got:\n%s", patch)
	}
}

func TestHunkPatch_newFile(t *testing.T) {
	hunk := Hunk{
		OldStart: 0,
		OldCount: 0,
		NewStart: 1,
		NewCount: 1,
		Header:   "@@ -0,0 +1 @@",
		Lines: []Line{
			{Type: LineAdded, Content: "new content", NewNum: 1},
		},
	}

	patch := HunkPatch("new.go", StatusAdded, hunk)

	if !strings.Contains(patch, "--- /dev/null\n") {
		t.Errorf("expected --- /dev/null for new file, got:\n%s", patch)
	}
	if !strings.Contains(patch, "+++ b/new.go\n") {
		t.Errorf("expected +++ b/new.go, got:\n%s", patch)
	}
}

func TestHunkPatch_deletedFile(t *testing.T) {
	hunk := Hunk{
		OldStart: 1,
		OldCount: 1,
		NewStart: 0,
		NewCount: 0,
		Header:   "@@ -1 +0,0 @@",
		Lines: []Line{
			{Type: LineRemoved, Content: "old content", OldNum: 1},
		},
	}

	patch := HunkPatch("old.go", StatusDeleted, hunk)

	if !strings.Contains(patch, "--- a/old.go\n") {
		t.Errorf("expected --- a/old.go, got:\n%s", patch)
	}
	if !strings.Contains(patch, "+++ /dev/null\n") {
		t.Errorf("expected +++ /dev/null for deleted file, got:\n%s", patch)
	}
}

func TestStageHunk(t *testing.T) {
	r := baseRepo(t)

	// Create unstaged changes
	r.WriteFile("base.go", "package main\n\n// added line\nfunc base() {}\n")

	// Get the unstaged diff to find the hunk
	raw, err := UnstagedDiff(DefaultContextLines)
	if err != nil {
		t.Fatal(err)
	}
	diff := Parse(raw)
	if len(diff.Files) == 0 {
		t.Fatal("expected unstaged changes")
	}
	file := diff.Files[0]
	if len(file.Hunks) == 0 {
		t.Fatal("expected at least one hunk")
	}

	// Stage the hunk
	err = StageHunk(file.Path, file.Status, file.Hunks[0])
	if err != nil {
		t.Fatalf("StageHunk: %v", err)
	}

	// Verify: should now be staged
	staged := r.StagedDiffRaw()
	if !strings.Contains(staged, "added line") {
		t.Errorf("expected staged diff to contain 'added line', got:\n%s", staged)
	}

	// Verify: should have no unstaged changes
	unstaged := r.UnstagedDiffRaw()
	if strings.Contains(unstaged, "added line") {
		t.Errorf("expected no unstaged changes for 'added line', got:\n%s", unstaged)
	}
}

func TestUnstageHunk(t *testing.T) {
	r := baseRepo(t)

	// Create and stage changes
	r.WriteFile("base.go", "package main\n\n// staged line\nfunc base() {}\n")
	r.Add("base.go")

	// Get the staged diff to find the hunk
	raw, err := StagedDiff(DefaultContextLines)
	if err != nil {
		t.Fatal(err)
	}
	diff := Parse(raw)
	if len(diff.Files) == 0 {
		t.Fatal("expected staged changes")
	}
	file := diff.Files[0]
	if len(file.Hunks) == 0 {
		t.Fatal("expected at least one hunk")
	}

	// Unstage the hunk
	err = UnstageHunk(file.Path, file.Status, file.Hunks[0])
	if err != nil {
		t.Fatalf("UnstageHunk: %v", err)
	}

	// Verify: should no longer be staged
	staged := r.StagedDiffRaw()
	if strings.Contains(staged, "staged line") {
		t.Errorf("expected no staged changes, got:\n%s", staged)
	}

	// Verify: should now be unstaged
	unstaged := r.UnstagedDiffRaw()
	if !strings.Contains(unstaged, "staged line") {
		t.Errorf("expected unstaged diff to contain 'staged line', got:\n%s", unstaged)
	}
}

func TestStageHunk_untrackedFile(t *testing.T) {
	r := baseRepo(t)

	// Create an untracked file
	r.WriteFile("newfile.go", "package main\n\nfunc newFunc() {}\n")

	// Simulate the synthetic hunk that UntrackedFiles() would produce
	hunk := Hunk{
		OldStart: 0,
		OldCount: 0,
		NewStart: 1,
		NewCount: 3,
		Header:   "@@ -0,0 +1,3 @@",
		Source:   SourceUnstaged,
		Lines: []Line{
			{Type: LineAdded, Content: "package main", NewNum: 1},
			{Type: LineAdded, Content: "", NewNum: 2},
			{Type: LineAdded, Content: "func newFunc() {}", NewNum: 3},
		},
	}

	err := StageHunk("newfile.go", StatusUntracked, hunk)
	if err != nil {
		t.Fatalf("StageHunk on untracked file: %v", err)
	}

	// Verify: should now be staged
	staged := r.StagedDiffRaw()
	if !strings.Contains(staged, "newFunc") {
		t.Errorf("expected staged diff to contain 'newFunc', got:\n%s", staged)
	}

	// Verify: should no longer appear as untracked
	status := r.StatusPorcelain()
	if strings.Contains(status, "??") {
		t.Errorf("expected file to no longer be untracked, got:\n%s", status)
	}
}

func TestStageFile(t *testing.T) {
	r := baseRepo(t)

	// Create unstaged changes
	r.WriteFile("base.go", "package main\n\n// stage me\nfunc base() {}\n")

	err := StageFile("base.go")
	if err != nil {
		t.Fatalf("StageFile: %v", err)
	}

	staged := r.StagedDiffRaw()
	if !strings.Contains(staged, "stage me") {
		t.Errorf("expected staged diff to contain 'stage me', got:\n%s", staged)
	}
	unstaged := r.UnstagedDiffRaw()
	if strings.Contains(unstaged, "stage me") {
		t.Errorf("expected no unstaged changes, got:\n%s", unstaged)
	}
}

func TestUnstageFile(t *testing.T) {
	r := baseRepo(t)

	// Create and stage changes
	r.WriteFile("base.go", "package main\n\n// unstage me\nfunc base() {}\n")
	r.Add("base.go")

	err := UnstageFile("base.go")
	if err != nil {
		t.Fatalf("UnstageFile: %v", err)
	}

	staged := r.StagedDiffRaw()
	if strings.Contains(staged, "unstage me") {
		t.Errorf("expected no staged changes, got:\n%s", staged)
	}
	unstaged := r.UnstagedDiffRaw()
	if !strings.Contains(unstaged, "unstage me") {
		t.Errorf("expected unstaged diff to contain 'unstage me', got:\n%s", unstaged)
	}
}

func TestDiscardFile(t *testing.T) {
	r := baseRepo(t)

	r.WriteFile("base.go", "package main\n\n// discard me\nfunc base() {}\n")

	err := DiscardFile("base.go", StatusModified, false)
	if err != nil {
		t.Fatalf("DiscardFile: %v", err)
	}

	status := r.StatusPorcelain()
	if status != "" {
		t.Errorf("expected clean working tree, got:\n%s", status)
	}
}

func TestDiscardFile_untracked(t *testing.T) {
	r := baseRepo(t)

	r.WriteFile("newfile.go", "package main\n")

	err := DiscardFile("newfile.go", StatusUntracked, false)
	if err != nil {
		t.Fatalf("DiscardFile: %v", err)
	}

	status := r.StatusPorcelain()
	if strings.Contains(status, "newfile.go") {
		t.Errorf("expected newfile.go to be gone, got:\n%s", status)
	}
}

func TestDiscardFile_staged(t *testing.T) {
	r := baseRepo(t)

	r.WriteFile("base.go", "package main\n\n// staged discard\nfunc base() {}\n")
	r.Add("base.go")

	err := DiscardFile("base.go", StatusModified, true)
	if err != nil {
		t.Fatalf("DiscardFile staged: %v", err)
	}

	status := r.StatusPorcelain()
	if status != "" {
		t.Errorf("expected clean working tree, got:\n%s", status)
	}
}

func TestDiscardHunk(t *testing.T) {
	r := baseRepo(t)

	// Create unstaged changes
	r.WriteFile("base.go", "package main\n\n// discard me\nfunc base() {}\n")

	// Get the unstaged diff to find the hunk
	raw, err := UnstagedDiff(DefaultContextLines)
	if err != nil {
		t.Fatal(err)
	}
	diff := Parse(raw)
	if len(diff.Files) == 0 {
		t.Fatal("expected unstaged changes")
	}
	file := diff.Files[0]
	if len(file.Hunks) == 0 {
		t.Fatal("expected at least one hunk")
	}

	// Discard the hunk
	err = DiscardHunk(file.Path, file.Status, file.Hunks[0])
	if err != nil {
		t.Fatalf("DiscardHunk: %v", err)
	}

	// Verify: should have no unstaged changes
	unstaged := r.UnstagedDiffRaw()
	if strings.Contains(unstaged, "discard me") {
		t.Errorf("expected no unstaged changes after discard, got:\n%s", unstaged)
	}

	// Verify: working tree file should be back to original
	status := r.StatusPorcelain()
	if status != "" {
		t.Errorf("expected clean working tree, got:\n%s", status)
	}
}

func TestDiscardHunk_staged(t *testing.T) {
	r := baseRepo(t)

	// Create and stage changes
	r.WriteFile("base.go", "package main\n\n// staged discard\nfunc base() {}\n")
	r.Add("base.go")

	// Get the staged diff to find the hunk
	raw, err := StagedDiff(DefaultContextLines)
	if err != nil {
		t.Fatal(err)
	}
	diff := Parse(raw)
	if len(diff.Files) == 0 {
		t.Fatal("expected staged changes")
	}
	file := diff.Files[0]
	if len(file.Hunks) == 0 {
		t.Fatal("expected at least one hunk")
	}

	// Discard the staged hunk — should unstage AND revert working tree
	hunk := file.Hunks[0]
	hunk.Source = SourceStaged
	err = DiscardHunk(file.Path, file.Status, hunk)
	if err != nil {
		t.Fatalf("DiscardHunk staged: %v", err)
	}

	// Verify: no staged changes
	staged := r.StagedDiffRaw()
	if strings.Contains(staged, "staged discard") {
		t.Errorf("expected no staged changes, got:\n%s", staged)
	}

	// Verify: no unstaged changes either (fully discarded)
	unstaged := r.UnstagedDiffRaw()
	if strings.Contains(unstaged, "staged discard") {
		t.Errorf("expected no unstaged changes, got:\n%s", unstaged)
	}

	// Verify: clean working tree
	status := r.StatusPorcelain()
	if status != "" {
		t.Errorf("expected clean working tree, got:\n%s", status)
	}
}

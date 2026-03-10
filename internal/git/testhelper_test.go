package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepo manages a temporary git repository for testing.
// Tests using Chdir must not call t.Parallel() — os.Chdir is process-global.
type TestRepo struct {
	Dir string
	t   *testing.T
}

// NewTestRepo creates a new git repo in a temp dir with "main" as the default branch.
// Sets local user.email and user.name so commits work without global git config.
func NewTestRepo(t *testing.T) *TestRepo {
	t.Helper()
	dir := t.TempDir()
	r := &TestRepo{Dir: dir, t: t}
	r.mustGit("init")
	r.mustGit("symbolic-ref", "HEAD", "refs/heads/main") // portable: works on all git versions
	r.mustGit("config", "user.email", "test@example.com")
	r.mustGit("config", "user.name", "Test")
	return r
}

// mustGit runs a git command in r.Dir, fataling the test on error.
func (r *TestRepo) mustGit(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// WriteFile writes content to a path relative to the repo root, creating directories as needed.
func (r *TestRepo) WriteFile(path, content string) {
	r.t.Helper()
	full := filepath.Join(r.Dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		r.t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		r.t.Fatalf("write %s: %v", path, err)
	}
}

// RemoveFile removes a file from the working tree.
func (r *TestRepo) RemoveFile(path string) {
	r.t.Helper()
	if err := os.Remove(filepath.Join(r.Dir, path)); err != nil {
		r.t.Fatalf("remove %s: %v", path, err)
	}
}

// Add stages the given paths.
func (r *TestRepo) Add(paths ...string) {
	r.t.Helper()
	r.mustGit(append([]string{"add"}, paths...)...)
}

// Commit creates a commit with currently staged changes.
func (r *TestRepo) Commit(message string) {
	r.t.Helper()
	r.mustGit("commit", "-m", message)
}

// CheckoutNewBranch creates and checks out a new branch from the current HEAD.
func (r *TestRepo) CheckoutNewBranch(branch string) {
	r.t.Helper()
	r.mustGit("checkout", "-b", branch)
}

// Chdir changes the process working directory to the repo root for the duration of the test.
// The original directory is restored in t.Cleanup.
func (r *TestRepo) Chdir() {
	r.t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		r.t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(r.Dir); err != nil {
		r.t.Fatalf("chdir %s: %v", r.Dir, err)
	}
	r.t.Cleanup(func() { os.Chdir(orig) })
}

// StagedDiffRaw returns the raw output of git diff --cached.
func (r *TestRepo) StagedDiffRaw() string {
	return r.mustGit("diff", "--cached")
}

// UnstagedDiffRaw returns the raw output of git diff.
func (r *TestRepo) UnstagedDiffRaw() string {
	return r.mustGit("diff")
}

// StatusPorcelain returns the output of git status --porcelain.
func (r *TestRepo) StatusPorcelain() string {
	return r.mustGit("status", "--porcelain")
}

// --- shared setup helpers ---

// baseRepo returns a TestRepo on "main" with one initial commit containing base.go.
// CWD is changed to the repo for the duration of the test.
func baseRepo(t *testing.T) *TestRepo {
	t.Helper()
	r := NewTestRepo(t)
	r.WriteFile("base.go", "package main\n\nfunc base() {}\n")
	r.Add("base.go")
	r.Commit("initial")
	r.Chdir()
	return r
}

// featureBranchRepo returns a TestRepo with:
//   - main: initial commit with base.go
//   - feature branch (checked out): one commit adding feature.go
//
// CWD is changed to the repo for the duration of the test.
func featureBranchRepo(t *testing.T) *TestRepo {
	t.Helper()
	r := NewTestRepo(t)
	r.WriteFile("base.go", "package main\n\nfunc base() {}\n")
	r.Add("base.go")
	r.Commit("initial")
	r.CheckoutNewBranch("feature")
	r.WriteFile("feature.go", "package main\n\nfunc feature() {}\n")
	r.Add("feature.go")
	r.Commit("add feature")
	r.Chdir()
	return r
}

// AddRemote adds a bare repo as "origin" so remote-based detection works.
// Returns the bare repo path.
func (r *TestRepo) AddRemote(t *testing.T, bareDir string) {
	t.Helper()
	r.AddRemoteAs(t, bareDir, "origin")
}

// AddRemoteAs adds a bare repo with the given remote name and pushes main to it.
func (r *TestRepo) AddRemoteAs(t *testing.T, bareDir, name string) {
	t.Helper()
	r.mustGit("remote", "add", name, bareDir)
	r.mustGit("push", "-u", name, "main")
}

// behindRemoteRepo returns a TestRepo on "main" where origin/main has a commit
// that the local main does not. Simulates being behind the remote.
func behindRemoteRepo(t *testing.T) *TestRepo {
	t.Helper()

	// Create the "remote" bare repo with main as default branch
	bareDir := t.TempDir()
	cmd := exec.Command("git", "init", "--bare", bareDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "symbolic-ref", "HEAD", "refs/heads/main")
	cmd.Dir = bareDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git symbolic-ref: %v\n%s", err, out)
	}

	// Create local repo, push initial commit
	r := NewTestRepo(t)
	r.WriteFile("base.go", "package main\n\nfunc base() {}\n")
	r.Add("base.go")
	r.Commit("initial")
	r.AddRemote(t, bareDir)

	// Clone into a second repo and push a new commit from there
	cloneDir := t.TempDir()
	cmd = exec.Command("git", "clone", bareDir, cloneDir)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = cloneDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = cloneDir
	cmd.Run()

	// Add a commit in the clone and push
	if err := os.WriteFile(filepath.Join(cloneDir, "remote-change.go"), []byte("package main\n\n// from remote\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "add", "remote-change.go")
	cmd.Dir = cloneDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", "remote commit")
	cmd.Dir = cloneDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "push")
	cmd.Dir = cloneDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git push: %v\n%s", err, out)
	}

	// Fetch in the original repo so origin/main is updated
	r.mustGit("fetch", "origin")

	r.Chdir()
	return r
}

// aheadOfRemoteRepo returns a TestRepo on "main" where local main has a commit
// that origin/main does not. Simulates being ahead of the remote.
func aheadOfRemoteRepo(t *testing.T) *TestRepo {
	t.Helper()

	bareDir := t.TempDir()
	cmd := exec.Command("git", "init", "--bare", bareDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "symbolic-ref", "HEAD", "refs/heads/main")
	cmd.Dir = bareDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git symbolic-ref: %v\n%s", err, out)
	}

	r := NewTestRepo(t)
	r.WriteFile("base.go", "package main\n\nfunc base() {}\n")
	r.Add("base.go")
	r.Commit("initial")
	r.AddRemote(t, bareDir)

	// Add a local commit (not pushed)
	r.WriteFile("local-change.go", "package main\n\n// local only\n")
	r.Add("local-change.go")
	r.Commit("local commit")

	r.Chdir()
	return r
}

// --- assertion helpers ---

func filePaths(diff *Diff) []string {
	paths := make([]string, len(diff.Files))
	for i, f := range diff.Files {
		paths[i] = f.Path
	}
	return paths
}

func assertHasContent(t *testing.T, f FileDiff, substr string) {
	t.Helper()
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			if strings.Contains(l.Content, substr) {
				return
			}
		}
	}
	t.Errorf("expected to find %q in diff lines for %s", substr, f.Path)
}

func fileByPath(diff *Diff, path string) *FileDiff {
	for i := range diff.Files {
		if diff.Files[i].Path == path {
			return &diff.Files[i]
		}
	}
	return nil
}

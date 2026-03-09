package comments

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Reply is a response from the agent to a review comment.
type Reply struct {
	Body      string `json:"body"`
	Timestamp string `json:"timestamp"`
}

// Comment represents a single code review comment with optional agent replies.
type Comment struct {
	ID      string  `json:"id"`
	File    string  `json:"file"`
	Line    int     `json:"line"`
	Side    string  `json:"side"` // "new" (added/context line) or "old" (removed line)
	Body    string  `json:"body"`
	Status  string  `json:"status"` // "open" or "closed"
	Replies []Reply `json:"replies,omitempty"`
}

// ReviewFile is the shared on-disk state between revise and Claude Code.
type ReviewFile struct {
	Repo      string    `json:"repo"`
	Branch    string    `json:"branch"`
	Submitted bool      `json:"submitted"`
	Comments  []Comment `json:"comments"`
}

// RepoRoot returns the root directory of the current git repository.
func RepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CurrentBranch returns the current git branch name.
// Falls back to the short commit hash for detached HEAD.
func CurrentBranch() (string, error) {
	out, err := exec.Command("git", "branch", "--show-current").Output()
	if err != nil {
		return "", fmt.Errorf("git branch --show-current: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch != "" {
		return branch, nil
	}
	// Detached HEAD — use short hash
	out, err = exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --short HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// FilePath derives the comment file path for the current git state.
func FilePath() (string, error) {
	root, err := RepoRoot()
	if err != nil {
		return "", err
	}
	branch, err := CurrentBranch()
	if err != nil {
		return "", err
	}
	return FilePathFor(root, branch), nil
}

// FilePathFor returns the deterministic comment file path for a repo/branch pair.
func FilePathFor(root, branch string) string {
	sum := sha256.Sum256([]byte(root + ":" + branch))
	return filepath.Join("/tmp", "revise", hex.EncodeToString(sum[:8])+".json")
}

// SocketPath returns the Unix socket path for the embedded MCP server.
func SocketPath(root, branch string) string {
	sum := sha256.Sum256([]byte(root + ":" + branch))
	return filepath.Join("/tmp", "revise", hex.EncodeToString(sum[:8])+".sock")
}

// Read reads the review file from disk. Returns nil if it doesn't exist.
func Read(path string) (*ReviewFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading review file: %w", err)
	}
	var rf ReviewFile
	if err := json.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("parsing review file: %w", err)
	}
	return &rf, nil
}

// Write writes the review file to disk atomically (write to temp, then rename).
func Write(path string, rf *ReviewFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating review dir: %w", err)
	}
	data, err := json.MarshalIndent(rf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling review file: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("renaming review file: %w", err)
	}
	return nil
}

// NewID generates a short random identifier for a comment.
func NewID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

// Timestamp returns the current time as an RFC3339 string.
func Timestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

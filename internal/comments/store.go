// Package comments provides persistence for review comments.
// Comments are stored as JSON in a temp directory, keyed by
// a hash of the repository root and branch name.
package comments

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// StorePath returns the file path for storing comments for the given repo and branch.
// The path is deterministic: /tmp/revise/<hash>.json where hash is derived from repo:branch.
func StorePath(repoRoot, branch string) string {
	h := sha256.Sum256([]byte(repoRoot + ":" + branch))
	hex := fmt.Sprintf("%x", h[:8]) // 16 hex chars
	return filepath.Join(os.TempDir(), "revise", hex+".json")
}

// Save writes the comments map to the given path as JSON.
// It creates parent directories if needed.
func Save(path string, entries map[string]string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Load reads the comments map from the given path.
// Returns an empty map (not an error) if the file does not exist.
func Load(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(map[string]string), nil
		}
		return nil, err
	}
	var entries map[string]string
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	if entries == nil {
		entries = make(map[string]string)
	}
	return entries, nil
}

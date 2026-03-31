package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// untrackedCache caches untracked file diffs to avoid re-reading large files
// on every context line change. The cache is invalidated when the untracked
// file list changes (different paths) or after a short TTL.
var untrackedCache struct {
	mu       sync.Mutex
	files    []FileDiff
	pathsKey string // sorted paths joined, used to detect changes
	time     time.Time
}

const untrackedCacheTTL = 2 * time.Second

// UntrackedFiles returns synthetic FileDiff entries for untracked files.
// Results are cached to avoid re-reading file contents when only context
// lines change (the bottleneck identified in issue #35).
func UntrackedFiles() ([]FileDiff, error) {
	out, err := exec.Command("git", "ls-files", "--others", "--exclude-standard").Output()
	if err != nil {
		return nil, err
	}

	rawPaths := strings.TrimSpace(string(out))

	untrackedCache.mu.Lock()
	defer untrackedCache.mu.Unlock()

	now := time.Now()
	if rawPaths == untrackedCache.pathsKey && now.Sub(untrackedCache.time) < untrackedCacheTTL {
		return untrackedCache.files, nil
	}

	paths := strings.Split(rawPaths, "\n")
	var files []FileDiff

	for _, path := range paths {
		if path == "" {
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// Detect binary files by checking for null bytes (same heuristic as git)
		if bytes.ContainsRune(content, 0) {
			files = append(files, FileDiff{
				Path:     path,
				Status:   StatusUntracked,
				IsBinary: true,
			})
			continue
		}

		lines := strings.Split(string(content), "\n")
		var hunkLines []Line
		for i, l := range lines {
			if i == len(lines)-1 && l == "" {
				continue // skip trailing empty line from split
			}
			hunkLines = append(hunkLines, Line{
				Type:    LineAdded,
				Content: l,
				NewNum:  i + 1,
			})
		}

		if len(hunkLines) == 0 {
			continue
		}

		files = append(files, FileDiff{
			Path:   path,
			Status: StatusUntracked,
			Hunks: []Hunk{
				{
					NewStart: 1,
					NewCount: len(hunkLines),
					Header:   fmt.Sprintf("@@ -0,0 +1,%d @@", len(hunkLines)),
					Source:   SourceUnstaged,
					Lines:    hunkLines,
				},
			},
		})
	}

	untrackedCache.files = files
	untrackedCache.pathsKey = rawPaths
	untrackedCache.time = now

	return files, nil
}

// InvalidateUntrackedCache forces the next UntrackedFiles call to re-read files.
func InvalidateUntrackedCache() {
	untrackedCache.mu.Lock()
	untrackedCache.time = time.Time{}
	untrackedCache.mu.Unlock()
}

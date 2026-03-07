package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// UntrackedFiles returns synthetic FileDiff entries for untracked files.
func UntrackedFiles() ([]FileDiff, error) {
	out, err := exec.Command("git", "ls-files", "--others", "--exclude-standard").Output()
	if err != nil {
		return nil, err
	}

	paths := strings.Split(strings.TrimSpace(string(out)), "\n")
	var files []FileDiff

	for _, path := range paths {
		if path == "" {
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
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
					Lines:    hunkLines,
				},
			},
		})
	}

	return files, nil
}

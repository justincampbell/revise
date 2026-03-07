package git

import (
	"fmt"
	"strconv"
	"strings"
)

// Parse parses a unified diff string into a Diff struct.
func Parse(raw string) *Diff {
	diff := &Diff{}
	if strings.TrimSpace(raw) == "" {
		return diff
	}

	lines := strings.Split(raw, "\n")
	var currentFile *FileDiff
	var currentHunk *Hunk
	var oldNum, newNum int

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// New file diff header
		if strings.HasPrefix(line, "diff --git ") {
			if currentFile != nil {
				if currentHunk != nil {
					currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
					currentHunk = nil
				}
				diff.Files = append(diff.Files, *currentFile)
			}
			currentFile = &FileDiff{}
			currentHunk = nil
			currentFile.Path = parseFilePath(line)
			currentFile.Status = StatusModified
			continue
		}

		if currentFile == nil {
			continue
		}

		// Detect file status from diff header lines
		if strings.HasPrefix(line, "new file mode") {
			currentFile.Status = StatusAdded
			continue
		}
		if strings.HasPrefix(line, "deleted file mode") {
			currentFile.Status = StatusDeleted
			continue
		}
		if strings.HasPrefix(line, "rename from ") {
			currentFile.OldPath = strings.TrimPrefix(line, "rename from ")
			currentFile.Status = StatusRenamed
			continue
		}
		if strings.HasPrefix(line, "rename to ") {
			currentFile.Path = strings.TrimPrefix(line, "rename to ")
			continue
		}

		// Skip index, --- and +++ lines
		if strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") {
			continue
		}

		// Hunk header
		if strings.HasPrefix(line, "@@") {
			if currentHunk != nil {
				currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
			}
			hunk := parseHunkHeader(line)
			currentHunk = &hunk
			oldNum = hunk.OldStart
			newNum = hunk.NewStart
			continue
		}

		if currentHunk == nil {
			continue
		}

		// Diff content lines
		if strings.HasPrefix(line, "+") {
			currentHunk.Lines = append(currentHunk.Lines, Line{
				Type:    LineAdded,
				Content: line[1:],
				NewNum:  newNum,
			})
			newNum++
		} else if strings.HasPrefix(line, "-") {
			currentHunk.Lines = append(currentHunk.Lines, Line{
				Type:    LineRemoved,
				Content: line[1:],
				OldNum:  oldNum,
			})
			oldNum++
		} else if strings.HasPrefix(line, " ") {
			currentHunk.Lines = append(currentHunk.Lines, Line{
				Type:    LineContext,
				Content: line[1:],
				OldNum:  oldNum,
				NewNum:  newNum,
			})
			oldNum++
			newNum++
		} else if line == `\ No newline at end of file` {
			// skip
		}
	}

	// Flush last file and hunk
	if currentFile != nil {
		if currentHunk != nil {
			currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
		}
		diff.Files = append(diff.Files, *currentFile)
	}

	return diff
}

// parseFilePath extracts the new file path from a "diff --git" header line.
// Handles both standard format ("diff --git a/path b/path") and
// no-prefix format produced when diff.noprefix=true ("diff --git path path").
func parseFilePath(line string) string {
	// Standard format: split on " b/" to get the new path.
	parts := strings.SplitN(line, " b/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	// No-prefix format (diff.noprefix=true): "diff --git path path"
	// The two paths are space-separated; take the first token after "diff --git ".
	rest := strings.TrimPrefix(line, "diff --git ")
	if idx := strings.Index(rest, " "); idx >= 0 {
		return rest[:idx]
	}
	return line
}

// parseHunkHeader parses "@@ -old,count +new,count @@ optional context" into a Hunk.
func parseHunkHeader(line string) Hunk {
	hunk := Hunk{Header: line}

	// Find the range info between @@ markers
	atIdx := strings.Index(line, "@@")
	if atIdx == -1 {
		return hunk
	}
	rest := line[atIdx+2:]
	endIdx := strings.Index(rest, "@@")
	if endIdx == -1 {
		return hunk
	}
	rangeStr := strings.TrimSpace(rest[:endIdx])

	parts := strings.Fields(rangeStr)
	for _, part := range parts {
		if strings.HasPrefix(part, "-") {
			old := parseRange(part[1:])
			hunk.OldStart = old[0]
			hunk.OldCount = old[1]
		} else if strings.HasPrefix(part, "+") {
			new := parseRange(part[1:])
			hunk.NewStart = new[0]
			hunk.NewCount = new[1]
		}
	}

	return hunk
}

// parseRange parses "start,count" or "start" into [start, count].
func parseRange(s string) [2]int {
	parts := strings.SplitN(s, ",", 2)
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return [2]int{1, 1}
	}
	count := 1
	if len(parts) == 2 {
		c, err := strconv.Atoi(parts[1])
		if err == nil {
			count = c
		}
	}
	return [2]int{start, count}
}

// FormatLine returns a formatted string for a line with gutter.
func FormatLine(l Line) string {
	switch l.Type {
	case LineAdded:
		return fmt.Sprintf("     %4d  +%s", l.NewNum, l.Content)
	case LineRemoved:
		return fmt.Sprintf("%4d       -%s", l.OldNum, l.Content)
	case LineContext:
		return fmt.Sprintf("%4d %4d   %s", l.OldNum, l.NewNum, l.Content)
	}
	return l.Content
}

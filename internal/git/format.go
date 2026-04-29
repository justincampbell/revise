package git

import (
	"fmt"
	"strings"
)

// Format reconstructs unified diff output from a parsed Diff.
func Format(d *Diff) string {
	var b strings.Builder
	for _, f := range d.Files {
		formatFile(&b, f)
	}
	return b.String()
}

// FormatHunks renders a Diff in the same shape the TUI shows on screen —
// file path header, hunk header with source label + function context, then
// each line prefixed by a 6-char line-number gutter and a +/-/space marker.
// No ANSI colors. Useful for non-interactive debugging via `revise diff --hunks`.
func FormatHunks(d *Diff) string {
	var b strings.Builder
	for i, f := range d.Files {
		if i > 0 {
			b.WriteString("\n")
		}
		formatHunksFile(&b, f)
	}
	return b.String()
}

func formatHunksFile(b *strings.Builder, f FileDiff) {
	fmt.Fprintf(b, "%s\n", f.Path)
	if f.IsBinary {
		b.WriteString("  Binary file — cannot display diff\n")
		return
	}
	for _, h := range f.Hunks {
		if header := HunkHeaderText(h); header != "" {
			fmt.Fprintf(b, "%s\n", header)
		}
		for _, l := range h.Lines {
			marker := " "
			switch l.Type {
			case LineAdded:
				marker = "+"
			case LineRemoved:
				marker = "-"
			}
			fmt.Fprintf(b, "%s%s %s\n", FormatGutter(l), marker, l.Content)
		}
	}
}

// FormatGutter returns the 6-char line-number gutter shown next to each diff
// line: 5-char right-aligned number + 1 space, or 6 spaces for non-content
// lines (hunk headers, blank separators). Removed lines use OldNum; added and
// context lines use NewNum.
func FormatGutter(l Line) string {
	n := l.NewNum
	if l.Type == LineRemoved {
		n = l.OldNum
	}
	switch l.Type {
	case LineAdded, LineRemoved, LineContext:
		return fmt.Sprintf("%5d ", n)
	}
	return "      "
}

// HunkHeaderText composes the hunk header the way it appears in the TUI:
// optional `[source]` tag (branch/staged/unstaged) + the function-context
// trailer from the `@@ -x,y +a,b @@ context` line. Returns "" when neither
// is available (e.g. file review mode).
func HunkHeaderText(h Hunk) string {
	label := HunkSourceLabel(h.Source)
	context := HunkContextText(h.Header)
	switch {
	case label == "" && context == "":
		return ""
	case context == "":
		return "[" + label + "]"
	case label == "":
		return context
	default:
		return "[" + label + "] " + context
	}
}

// HunkSourceLabel returns the lowercase label for a HunkSource ("branch",
// "staged", "unstaged"), or "" for the zero value.
func HunkSourceLabel(s HunkSource) string {
	switch s {
	case SourceBranch:
		return "branch"
	case SourceStaged:
		return "staged"
	case SourceUnstaged:
		return "unstaged"
	}
	return ""
}

// HunkContextText extracts the trailing context from a unified-diff hunk
// header (`@@ -x,y +a,b @@ trailing context`). Returns "" if the header has
// no trailing context.
func HunkContextText(header string) string {
	parts := strings.SplitN(header, "@@", 3)
	if len(parts) < 3 {
		return strings.TrimSpace(header)
	}
	return strings.TrimSpace(parts[2])
}

func formatFile(b *strings.Builder, f FileDiff) {
	path := f.Path
	oldPath := f.OldPath
	if oldPath == "" {
		oldPath = path
	}

	fmt.Fprintf(b, "diff --git a/%s b/%s\n", oldPath, path)

	switch f.Status {
	case StatusAdded:
		b.WriteString("new file mode 100644\n")
		fmt.Fprintf(b, "--- /dev/null\n")
		fmt.Fprintf(b, "+++ b/%s\n", path)
	case StatusDeleted:
		b.WriteString("deleted file mode 100644\n")
		fmt.Fprintf(b, "--- a/%s\n", oldPath)
		fmt.Fprintf(b, "+++ /dev/null\n")
	case StatusRenamed:
		fmt.Fprintf(b, "rename from %s\n", oldPath)
		fmt.Fprintf(b, "rename to %s\n", path)
		fmt.Fprintf(b, "--- a/%s\n", oldPath)
		fmt.Fprintf(b, "+++ b/%s\n", path)
	default:
		fmt.Fprintf(b, "--- a/%s\n", oldPath)
		fmt.Fprintf(b, "+++ b/%s\n", path)
	}

	for _, h := range f.Hunks {
		fmt.Fprintf(b, "@@ -%d,%d +%d,%d @@\n", h.OldStart, h.OldCount, h.NewStart, h.NewCount)
		for _, l := range h.Lines {
			switch l.Type {
			case LineContext:
				fmt.Fprintf(b, " %s\n", l.Content)
			case LineAdded:
				fmt.Fprintf(b, "+%s\n", l.Content)
			case LineRemoved:
				fmt.Fprintf(b, "-%s\n", l.Content)
			}
		}
	}
}

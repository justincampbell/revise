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

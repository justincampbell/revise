package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/justincampbell/revise/internal/git"
)

// padVisual pads s with spaces to reach the desired visible width,
// accounting for ANSI escape sequences.
func padVisual(s string, width int) string {
	visible := ansi.StringWidth(s)
	if visible >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visible)
}

// PrintStylesDemo prints a matrix of file status indicators across all
// staging combinations, for visual verification of color choices.
func PrintStylesDemo() {
	statuses := []git.FileStatus{
		git.StatusModified,
		git.StatusAdded,
		git.StatusDeleted,
		git.StatusRenamed,
		git.StatusUntracked,
	}

	stagingStates := []struct {
		name    string
		staging stagingSources
	}{
		{"branch", stagingSources{branch: true}},
		{"unstaged", stagingSources{unstaged: true}},
		{"staged", stagingSources{staged: true}},
		{"partial", stagingSources{staged: true, unstaged: true}},
		{"br+unstg", stagingSources{branch: true, unstaged: true}},
		{"br+stg", stagingSources{branch: true, staged: true}},
	}

	col := 10

	// Header
	header := padVisual("", 14)
	for _, ss := range stagingStates {
		header += padVisual(ss.name, col)
	}
	fmt.Println(header)
	fmt.Println(strings.Repeat("─", 14+col*len(stagingStates)))

	// Rows
	for _, s := range statuses {
		row := padVisual(string(s)+" "+statusName(s), 14)
		for _, ss := range stagingStates {
			indicator := statusIndicator(s, ss.staging)
			row += padVisual(indicator, col)
		}
		fmt.Println(row)
	}

	// Legend
	fmt.Println()
	fmt.Println("Legend:")
	fmt.Println("  " + statusDimModified.Render("■") + " dim     = branch only (committed)")
	fmt.Println("  " + statusModified.Render("■") + " yellow  = unstaged/default")
	fmt.Println("  " + statusAdded.Render("■") + " green   = fully staged")
	fmt.Println("  " + statusPartiallyStaged.Render("■") + " cyan    = partially staged")
	fmt.Println("  " + statusDeleted.Render("■") + " red     = deleted")
	fmt.Println("  " + statusUntracked.Render("■") + " cyan    = untracked")
}

func statusName(s git.FileStatus) string {
	switch s {
	case git.StatusModified:
		return "Modified"
	case git.StatusAdded:
		return "Added"
	case git.StatusDeleted:
		return "Deleted"
	case git.StatusRenamed:
		return "Renamed"
	case git.StatusUntracked:
		return "Untracked"
	default:
		return ""
	}
}

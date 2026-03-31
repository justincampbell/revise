package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"charm.land/lipgloss/v2"
	"github.com/justincampbell/revise/internal/comments"
	"github.com/justincampbell/revise/internal/git"
	"github.com/justincampbell/revise/internal/ui"
	"github.com/justincampbell/revise/internal/update"
)

var version = "dev"

// ttyOut is the TUI output writer. Opened from /dev/tty so the TUI
// renders to the terminal even when stdout is redirected.
var ttyOut *os.File

func init() {
	// Open /dev/tty so the TUI renders to the real terminal even when
	// stdout is redirected (enables piping and EDITOR workflows).
	var err error
	ttyOut, err = os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		ttyOut = os.Stderr
	}
}

func main() {

	outputFlag := flag.String("output", "", "Write comments to file on exit")
	helpFlag := flag.Bool("help", false, "Show help")
	versionFlag := flag.Bool("version", false, "Show version")
	flag.BoolVar(versionFlag, "v", false, "Show version (shorthand)")
	themeFlag := flag.String("theme", "auto", "Color theme: auto, auto-daltonized, charmtone-dark, charmtone-dark-daltonized, charmtone-light, charmtone-light-daltonized, github-dark, github-dark-daltonized, github-light, github-light-daltonized")
	flag.Parse()

	if *helpFlag {
		printHelp()
		os.Exit(0)
	}

	if *versionFlag {
		fmt.Println("revise", version)
		os.Exit(0)
	}

	theme := ui.Theme(*themeFlag)
	if !ui.IsValidTheme(theme) {
		valid := make([]string, len(ui.ValidThemes))
		for i, t := range ui.ValidThemes {
			valid[i] = string(t)
		}
		fmt.Fprintf(os.Stderr, "Error: unknown theme %q (valid: %s)\n", *themeFlag, strings.Join(valid, ", "))
		os.Exit(1)
	}
	var isDark bool
	switch theme {
	case ui.ThemeAuto, ui.ThemeAutoDaltonized:
		isDark = lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
	default:
		isDark = ui.IsDarkTheme(theme)
	}
	ui.SetTheme(theme, isDark)

	// Subcommands that don't require a git repo.
	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "update":
			runUpdate(args[1:])
			return
		case "styles":
			ui.PrintStylesDemo()
			return
		case "diff":
			// Handled below after git repo check.
		default:
			// Positional argument: file review mode.
			runFileReview(args[0], *outputFlag)
			return
		}
	}

	// All remaining paths require a git repo with commits.
	if !git.IsGitRepo() {
		fmt.Fprintln(os.Stderr, "Error: not a git repository")
		os.Exit(1)
	}

	if !git.HasCommits() {
		fmt.Fprintln(os.Stderr, "Error: repository has no commits")
		os.Exit(1)
	}

	// Subcommands that require a git repo.
	if len(args) > 0 && args[0] == "diff" {
		runDiff()
		return
	}

	onDefaultBranch, err := git.IsOnDefaultBranch()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var diff *git.Diff
	if onDefaultBranch {
		diff, err = git.WorkingTreeDiff(git.DefaultContextLines)
	} else {
		diff, err = git.BranchDiff(git.DefaultContextLines)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Compute the store path for comment persistence.
	var storePath string
	if repoRoot, err := git.RepoRoot(); err == nil {
		branch := git.CurrentBranchName()
		if branch == "" {
			branch = "HEAD"
		}
		storePath = comments.StorePath(repoRoot, branch)
	}

	m := ui.NewWithStorePath(diff, onDefaultBranch, storePath, version)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithReportFocus(), tea.WithOutput(ttyOut))

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Output comments on exit.
	if fm, ok := finalModel.(ui.Model); ok {
		writeComments(fm, *outputFlag)
	}
}

func runFileReview(filePath string, outputPath string) {
	diff, err := buildFileDiff(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	m := ui.NewFileReview(diff, filePath)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithOutput(ttyOut))
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Output comments on exit.
	if fm, ok := finalModel.(ui.Model); ok {
		writeComments(fm, outputPath)
	}
}

// writeComments outputs comments to --output file or stdout.
func writeComments(m ui.Model, outputPath string) {
	text := m.ExportedComments()
	if text == "" {
		return
	}
	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(text), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing comments: %v\n", err)
		}
		return
	}
	fmt.Print(text)
}

// buildFileDiff reads a file and returns a synthetic Diff for file review mode.
func buildFileDiff(filePath string) (*git.Diff, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // best-effort close on read-only file

	var lines []git.Line
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		lines = append(lines, git.Line{
			Type:    git.LineContext,
			Content: scanner.Text(),
			OldNum:  lineNum,
			NewNum:  lineNum,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &git.Diff{
		Files: []git.FileDiff{{
			Path:   filePath,
			Status: git.StatusModified,
			Hunks:  []git.Hunk{{Lines: lines}},
		}},
	}, nil
}

func runDiff() {
	diff, err := git.GetDiff()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(git.Format(diff))
}

func runUpdate(args []string) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	pre := fs.Bool("pre", false, "Include pre-release (dev) builds")
	_ = fs.Parse(args)

	fmt.Printf("Current version: %s\n", version)
	fmt.Println("Checking for updates...")

	info, err := update.CheckForUpdate(version, *pre)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if info == nil || !info.IsNewer {
		fmt.Printf("Already up to date (%s)\n", version)
		return
	}

	fmt.Printf("Updating to %s...\n", info.LatestVersion)
	if err := update.ApplyUpdate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Updated to %s\n", info.LatestVersion)
}

func printHelp() {
	fmt.Println(`revise - Review local git changes

Usage: revise [flags] [command] [file]

Flags:
  --help                Show this help
  --version, -v         Show version
  --theme <name>        Color theme (default: auto)
                          auto, auto-daltonized
                          charmtone-dark, charmtone-dark-daltonized
                          charmtone-light, charmtone-light-daltonized
                          github-dark, github-dark-daltonized
                          github-light, github-light-daltonized
  --output <file>       Write comments to file on exit

Commands:
  diff            Print unified diff (no TUI)
  styles          Show file status color matrix
  update [--pre]  Update to the latest version

File review:
  revise <file>   Review a file with comments`)
}

package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"charm.land/lipgloss/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/justincampbell/revise/internal/comments"
	"github.com/justincampbell/revise/internal/devwatch"
	"github.com/justincampbell/revise/internal/fswatch"
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
	devFlag := flag.Bool("dev", false, "Auto-restart when the binary is replaced (for local development)")
	debugFlag := flag.Bool("debug", false, "Show a refresh-debug strip with live timings above the status bar")
	noWatchFlag := flag.Bool("no-watch", false, "Disable fsnotify-based refresh; auto-refresh runs on a timer only")
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
		case "diff", "setup-cache":
			// Handled below after git repo check.
		default:
			// Positional argument: file review mode.
			runFileReview(args[0], *outputFlag, *devFlag)
			return
		}
	}

	// All remaining paths require a git repo. Repos with no commits are
	// supported — the TUI surfaces staged + untracked files and a status
	// hint that no commits exist yet.
	if !git.IsGitRepo() {
		fmt.Fprintln(os.Stderr, "Error: not a git repository")
		os.Exit(1)
	}

	// Subcommands that require a git repo.
	if len(args) > 0 && args[0] == "diff" {
		runDiff(args[1:])
		return
	}
	if len(args) > 0 && args[0] == "setup-cache" {
		runSetupCache()
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
		// Overlap-collapse is on by default (matches the model's initial state),
		// so the first paint matches what the UI shows before any reload.
		diff, _, err = git.BranchDiffDepth(git.DefaultContextLines, false, 0, true)
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

	m := ui.NewWithStorePath(diff, onDefaultBranch, storePath, version).
		WithDebug(*debugFlag)

	// Attach an fsnotify-based watcher so file edits wake the refresh
	// loop directly (instead of waiting for the next scheduled tick).
	// Soft-fail to timer-only on any error — fsnotify can fail on
	// network filesystems, FD-limited environments, or unusual repos.
	if !*noWatchFlag && os.Getenv("REVISE_NO_WATCH") == "" {
		if watcher, werr := startWatcher(); werr == nil {
			defer watcher.Close() //nolint:errcheck // best-effort on shutdown
			m = m.WithFSWatcher(watcher)
		}
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithReportFocus(), tea.WithOutput(ttyOut))

	finalModel, err := runProgram(p, *devFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Output comments on exit.
	if fm, ok := finalModel.(ui.Model); ok {
		writeComments(fm, *outputFlag)
	}
}

func runFileReview(filePath string, outputPath string, devMode bool) {
	diff, err := buildFileDiff(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	m := ui.NewFileReview(diff, filePath)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithOutput(ttyOut))
	finalModel, err := runProgram(p, devMode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Output comments on exit.
	if fm, ok := finalModel.(ui.Model); ok {
		writeComments(fm, outputPath)
	}
}

// startWatcher initializes the fswatch.Watcher pointed at the current
// repo's working tree and git directory. Returns an error if either
// path lookup or fsnotify init fails — callers fall back to timer-only.
func startWatcher() (*fswatch.Watcher, error) {
	workTree, err := git.RepoRoot()
	if err != nil {
		return nil, err
	}
	gitDir, err := git.GitDir()
	if err != nil {
		return nil, err
	}
	return fswatch.New(workTree, gitDir, 100*time.Millisecond)
}

// runProgram runs a Bubbletea program. When devMode is true, it polls the
// running binary for changes and re-execs the process when the binary is
// replaced (e.g. after `make install`), preserving the original argv and env.
func runProgram(p *tea.Program, devMode bool) (tea.Model, error) {
	var (
		restart  atomic.Bool
		selfPath string
	)
	if devMode {
		if exe, err := os.Executable(); err == nil {
			if real, err := filepath.EvalSymlinks(exe); err == nil {
				selfPath = real
			} else {
				selfPath = exe
			}
		}
		if selfPath != "" {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			w := devwatch.New(selfPath, func() {
				restart.Store(true)
				p.Quit()
			})
			go w.Run(ctx)
		}
	}

	finalModel, err := p.Run()
	if restart.Load() && selfPath != "" {
		// Re-exec the (now-updated) binary in place. On success, this never
		// returns; on failure, fall through and surface the original result.
		_ = syscall.Exec(selfPath, os.Args, os.Environ())
	}
	return finalModel, err
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

func runDiff(args []string) {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	modeFlag := fs.String("mode", "", "Diff mode: branch, staged, staged-only, unstaged (default: auto-detect)")
	hunksFlag := fs.Bool("hunks", false, "Print TUI-style hunks (file path header, line-number gutter) instead of unified diff")
	overlapFlag := fs.Bool("overlap", false, "Branch mode: collapse committed + working-tree edits to the same lines into one hunk")
	_ = fs.Parse(args)

	diff, err := loadDiffForMode(*modeFlag, *overlapFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *hunksFlag {
		fmt.Print(git.FormatHunks(diff))
		return
	}
	fmt.Print(git.Format(diff))
}

// loadDiffForMode returns a *git.Diff for the given mode string. An empty
// mode auto-detects (branch on feature branches, working-tree on default).
func loadDiffForMode(mode string, overlap bool) (*git.Diff, error) {
	switch mode {
	case "":
		// Mirror GetDiff's auto-detection, but thread the overlap flag through
		// the Branch-mode path (GetDiff itself doesn't take it).
		onDefault, err := git.IsOnDefaultBranch()
		if err != nil {
			return nil, err
		}
		if onDefault {
			return git.WorkingTreeDiff(git.DefaultContextLines)
		}
		diff, _, err := git.BranchDiffDepth(git.DefaultContextLines, false, 0, overlap)
		return diff, err
	case "branch":
		diff, _, err := git.BranchDiffDepth(git.DefaultContextLines, false, 0, overlap)
		return diff, err
	case "staged":
		return git.WorkingTreeDiff(git.DefaultContextLines)
	case "staged-only":
		return git.StagedOnlyDiff(git.DefaultContextLines)
	case "unstaged":
		return git.UnstagedOnlyDiff(git.DefaultContextLines)
	default:
		return nil, fmt.Errorf("unknown --mode %q (valid: branch, staged, staged-only, unstaged)", mode)
	}
}

func runSetupCache() {
	if git.UntrackedCacheEnabled() {
		fmt.Println("core.untrackedCache is already enabled")
		return
	}
	if err := git.TestUntrackedCacheSupport(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if err := git.EnableUntrackedCache(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("enabled core.untrackedCache for this repo")
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
  --dev                 Auto-restart when the binary is replaced (development)
  --debug               Show a refresh-debug strip with live timings above the status bar
  --no-watch            Disable fsnotify-based refresh (timer-only auto-refresh)

Commands:
  diff [flags]    Print diff (no TUI)
                    --mode branch | staged | staged-only | unstaged
                      (default: auto-detect — same as launching the TUI)
                    --hunks  TUI-style output (path header, line-number gutter)
                      instead of unified diff format
  setup-cache     Enable git's core.untrackedCache for faster refreshes
  styles          Show file status color matrix
  update [--pre]  Update to the latest version

File review:
  revise <file>   Review a file with comments`)
}

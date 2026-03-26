package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justincampbell/revise/internal/comments"
	"github.com/justincampbell/revise/internal/git"
	"github.com/justincampbell/revise/internal/ui"
	"github.com/justincampbell/revise/internal/update"
)

var version = "dev"

func main() {
	helpFlag := flag.Bool("help", false, "Show help")
	versionFlag := flag.Bool("version", false, "Show version")
	flag.BoolVar(versionFlag, "v", false, "Show version (shorthand)")
	themeFlag := flag.String("theme", "dark", "Color theme: dark, light, dark-daltonized, light-daltonized")
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
	ui.SetTheme(theme)

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
	if len(args) > 0 {
		switch args[0] {
		case "diff":
			runDiff()
			return
		default:
			fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
			os.Exit(1)
		}
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
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithReportFocus())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
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
	fs.Parse(args)

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

Usage: revise [flags] [command]

Flags:
  --help                Show this help
  --version, -v         Show version
  --theme <name>        Color theme: dark (default), light, dark-daltonized, light-daltonized

Commands:
  diff            Print unified diff (no TUI)
  styles          Show file status color matrix
  update [--pre]  Update to the latest version`)
}

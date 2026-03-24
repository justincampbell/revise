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
	flag.Parse()

	if *helpFlag {
		printHelp()
		os.Exit(0)
	}

	if *versionFlag {
		fmt.Println("revise", version)
		os.Exit(0)
	}

	// Subcommand routing — before git repo checks.
	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "update":
			runUpdate(args[1:])
			return
		default:
			fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
			os.Exit(1)
		}
	}

	if !git.IsGitRepo() {
		fmt.Fprintln(os.Stderr, "Error: not a git repository")
		os.Exit(1)
	}

	if !git.HasCommits() {
		fmt.Fprintln(os.Stderr, "Error: repository has no commits")
		os.Exit(1)
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
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
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
  --help           Show this help
  --version, -v    Show version

Commands:
  update [--pre]  Update to the latest version`)

	for _, group := range ui.BindingGroups() {
		fmt.Printf("\n%s:\n", group.Name)
		for _, b := range group.Bindings {
			pad := 14 - len([]rune(b.Key))
			if pad < 1 {
				pad = 1
			}
			fmt.Printf("  %s%s %s\n", b.Key, strings.Repeat(" ", pad), b.Desc)
		}
	}
}

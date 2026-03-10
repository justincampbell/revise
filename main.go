package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justincampbell/revise/internal/git"
	"github.com/justincampbell/revise/internal/ui"
)

var version = "dev"

func main() {
	helpFlag := flag.Bool("help", false, "Show help")
	versionFlag := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *helpFlag {
		printHelp()
		os.Exit(0)
	}

	if *versionFlag {
		fmt.Println("revise", version)
		os.Exit(0)
	}

	if !git.IsGitRepo() {
		fmt.Fprintln(os.Stderr, "Error: not a git repository")
		os.Exit(1)
	}

	if !git.HasCommits() {
		fmt.Fprintln(os.Stderr, "Error: repository has no commits")
		os.Exit(1)
	}

	diff, err := git.GetDiff()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	onDefaultBranch, err := git.IsOnDefaultBranch()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	m := ui.New(diff, onDefaultBranch)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(`revise - Review local git changes

Usage: revise [flags]

Flags:
  --help      Show this help
  --version   Show version`)

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

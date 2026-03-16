package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justincampbell/revise/internal/comments"
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

	// File review mode: positional argument
	if args := flag.Args(); len(args) > 0 {
		filePath := args[0]
		diff, err := buildFileDiff(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		m := ui.NewFileReview(diff, filePath)
		p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
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

	m := ui.NewWithStorePath(diff, onDefaultBranch, storePath)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// buildFileDiff reads a file and returns a synthetic Diff for file review mode.
func buildFileDiff(filePath string) (*git.Diff, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

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

func printHelp() {
	fmt.Println(`revise - Review local git changes

Usage: revise [flags] [file]

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

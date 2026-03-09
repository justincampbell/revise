package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	revisecomments "github.com/justincampbell/revise/internal/comments"
	"github.com/justincampbell/revise/internal/git"
	"github.com/justincampbell/revise/internal/mcp"
	"github.com/justincampbell/revise/internal/ui"
)

var version = "dev"

func main() {
	// Handle subcommands before flag parsing.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "mcp":
			runMCP()
			return
		case "path":
			runPath()
			return
		}
	}

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

	if len(diff.Files) == 0 {
		fmt.Println("No changes found")
		os.Exit(0)
	}

	if err := runTUI(diff); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runTUI(diff *git.Diff) error {
	// Derive review file and socket paths (best-effort).
	var reviewFilePath, socketPath string
	if root, err := revisecomments.RepoRoot(); err == nil {
		if branch, err := revisecomments.CurrentBranch(); err == nil {
			reviewFilePath = revisecomments.FilePathFor(root, branch)
			socketPath = revisecomments.SocketPath(root, branch)
		}
	}

	// Create the MCP server.
	var mcpServer *mcp.Server
	if reviewFilePath != "" {
		mcpServer = mcp.New(reviewFilePath)
	}

	m := ui.New(diff)
	if mcpServer != nil {
		m.SetMCPServer(mcpServer)
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Wire MCP notifications to the tea program.
	if mcpServer != nil {
		mcpServer.SetNotify(func(msg interface{}) { p.Send(msg) })
		go func() {
			if err := mcpServer.ListenUnix(socketPath); err != nil {
				// Non-fatal: MCP server failure doesn't stop the TUI.
				_ = err
			}
		}()
	}

	_, err := p.Run()
	return err
}

// runMCP connects to a running TUI's embedded MCP socket if available,
// otherwise falls back to a standalone file-polling MCP server on stdio.
func runMCP() {
	root, err := revisecomments.RepoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: "+err.Error())
		os.Exit(1)
	}

	branch, err := revisecomments.CurrentBranch()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: "+err.Error())
		os.Exit(1)
	}

	socketPath := revisecomments.SocketPath(root, branch)

	// Try to connect to the running TUI's embedded MCP socket.
	if conn, err := net.DialTimeout("unix", socketPath, 200*time.Millisecond); err == nil {
		go io.Copy(conn, os.Stdin)
		io.Copy(os.Stdout, conn)
		return
	}

	// TUI not running — fall back to standalone file-polling server.
	reviewFilePath := revisecomments.FilePathFor(root, branch)
	server := mcp.New(reviewFilePath)
	if err := server.ServeStdio(); err != nil {
		fmt.Fprintln(os.Stderr, "Error: "+err.Error())
		os.Exit(1)
	}
}

// runPath prints the review JSON file path for the current repo/branch and exits.
func runPath() {
	reviewFilePath, err := revisecomments.FilePath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: "+err.Error())
		os.Exit(1)
	}
	fmt.Println(reviewFilePath)
}

func printHelp() {
	fmt.Println(`revise - Review local git changes

Usage: revise [flags]
       revise mcp      Run MCP server on stdio (for Claude Code integration)
       revise path     Print the review file path for the current repo/branch

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

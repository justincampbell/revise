package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/justincampbell/revise/internal/comments"
	"github.com/justincampbell/revise/internal/config"
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
	// Mutate the existing default renderer (don't replace it) so that
	// styles already created by imported packages keep their reference.
	var err error
	ttyOut, err = os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		ttyOut = os.Stderr
	}
	r := lipgloss.DefaultRenderer()
	r.SetOutput(termenv.NewOutput(ttyOut))
}

func main() {

	outputFlag := flag.String("output", "", "Write comments to file on exit")
	helpFlag := flag.Bool("help", false, "Show help")
	versionFlag := flag.Bool("version", false, "Show version")
	flag.BoolVar(versionFlag, "v", false, "Show version (shorthand)")
	themeFlag := flag.String("theme", "dark", "Color theme: dark, light, dark-daltonized, light-daltonized")
	flag.Parse()

	if *helpFlag {
		printHelp()
		os.Exit(0)
	}

	// Load config from file.
	cfg, cfgWarnings, cfgErr := config.LoadDefault()
	if cfgErr != nil {
		cfgWarnings = append(cfgWarnings, fmt.Sprintf("config: %v", cfgErr))
	}
	resolved := config.Resolve(cfg)

	// CLI flags override config values.
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "theme":
			resolved.Theme = *themeFlag
		}
	})

	if *versionFlag {
		configInfo := "(no config file)"
		if config.Exists() {
			configInfo = fmt.Sprintf("(config: %s)", config.Path())
		}
		fmt.Println("revise", version, configInfo)
		os.Exit(0)
	}

	// Validate theme.
	theme := ui.Theme(resolved.Theme)
	if !ui.IsValidTheme(theme) {
		valid := make([]string, len(ui.ValidThemes))
		for i, t := range ui.ValidThemes {
			valid[i] = string(t)
		}
		cfgWarnings = append(cfgWarnings, fmt.Sprintf("invalid theme %q (valid: %s), using default", resolved.Theme, strings.Join(valid, ", ")))
		theme = ui.ThemeDark
	}
	ui.SetTheme(theme)

	// Subcommands that don't require a git repo.
	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "config":
			runConfig(args[1:])
			return
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

	// Map config default_mode to DiffMode.
	defaultMode := parseDiffMode(resolved.DefaultMode)

	contextLines := resolved.ContextLines
	hideWhitespace := !resolved.Whitespace

	var diff *git.Diff
	if onDefaultBranch {
		diff, err = git.WorkingTreeDiffOptions(contextLines, hideWhitespace)
	} else {
		diff, err = git.BranchDiffOptions(contextLines, hideWhitespace)
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

	m := ui.NewFromOptions(ui.ModelOptions{
		Diff:            diff,
		OnDefaultBranch: onDefaultBranch,
		StorePath:       storePath,
		Version:         version,
		ContextLines:    contextLines,
		HideWhitespace:  hideWhitespace,
		UpdateCheck:     resolved.UpdateCheck,
		DefaultMode:     defaultMode,
		ConfigWarnings:  cfgWarnings,
	})

	opts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithReportFocus(), tea.WithOutput(ttyOut)}
	if resolved.Mouse {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	p := tea.NewProgram(m, opts...)

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

func parseDiffMode(s string) ui.DiffMode {
	switch s {
	case "staged":
		return ui.ModeStaged
	case "staged_only":
		return ui.ModeStagedOnly
	case "unstaged_only":
		return ui.ModeUnstaged
	default:
		return ui.ModeBranch
	}
}

func runConfig(args []string) {
	if len(args) == 0 {
		// Bare "revise config" — print resolved config.
		cfg, _, _ := config.LoadDefault()
		resolved := config.Resolve(cfg)
		p := config.Path()
		if p != "" {
			fmt.Printf("# %s\n", p)
		}
		fmt.Printf("theme: %s\n", resolved.Theme)
		fmt.Printf("default_mode: %s\n", resolved.DefaultMode)
		fmt.Printf("context_lines: %d\n", resolved.ContextLines)
		fmt.Printf("whitespace: %t\n", resolved.Whitespace)
		fmt.Printf("mouse: %t\n", resolved.Mouse)
		fmt.Printf("update_check: %s\n", resolved.UpdateCheck)
		return
	}

	switch args[0] {
	case "path":
		p := config.Path()
		if p == "" {
			fmt.Fprintln(os.Stderr, "Error: cannot determine config path (HOME not set)")
			os.Exit(1)
		}
		fmt.Println(p)

	case "init":
		fs := flag.NewFlagSet("config init", flag.ExitOnError)
		force := fs.Bool("force", false, "Overwrite existing config file")
		fs.Parse(args[1:])

		p := config.Path()
		if p == "" {
			fmt.Fprintln(os.Stderr, "Error: cannot determine config path (HOME not set)")
			os.Exit(1)
		}
		if err := config.WriteTemplate(p, *force); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Created config file at %s\n", p)

	default:
		fmt.Fprintf(os.Stderr, "unknown config command: %s\n", args[0])
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

Usage: revise [flags] [command] [file]

Flags:
  --help                Show this help
  --version, -v         Show version
  --theme <name>        Color theme: dark (default), light, dark-daltonized, light-daltonized
  --output <file>       Write comments to file on exit

Commands:
  config          Show, create, or manage configuration
  config path     Print config file path
  config init     Create default config file (--force to overwrite)
  diff            Print unified diff (no TUI)
  styles          Show file status color matrix
  update [--pre]  Update to the latest version

File review:
  revise <file>   Review a file with comments`)
}

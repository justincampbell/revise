# revise

A terminal UI for reviewing local git changes and sending feedback to [Claude Code](https://docs.anthropic.com/en/docs/claude-code).

<!-- TODO: Add demo video/gif -->

`revise` gives you a split-pane interface for browsing git diffs in the terminal — file list on the left, diff on the right. Add inline comments, stage or discard individual hunks, and cycle between branch, staged, and unstaged views. It's designed for reviewing your own changes before committing or sharing them with an AI assistant.

## Installation

### Homebrew

```sh
brew install justincampbell/tap/revise
```

### Go

```sh
go install github.com/justincampbell/revise@latest
```

### Binary releases

Download a prebuilt binary from the [GitHub Releases](https://github.com/justincampbell/revise/releases) page.

## Usage

Run from within a git repository:

```sh
revise
```

- On a **feature branch**, revise shows all changes compared to the merge base (as if opening a PR).
- On the **default branch**, it shows staged, unstaged, and untracked changes.

### Diff Modes

Cycle through modes with `Tab` / `Shift+Tab`:

| Mode | What it shows |
|------|--------------|
| **Branch** | Committed + staged + unstaged + untracked (broadest; default on feature branches) |
| **Staged** | Staged + unstaged + untracked |
| **Unstaged** | Unstaged + untracked only (narrowest) |

In **Branch** mode, filter the committed range to the last *N* commits with `<` / `>` instead of reviewing everything since the merge-base. `<` steps back in time (the most recent commit first, then one further back with each press); `>` steps forward (from the full branch it drops the oldest commit first, then continues toward the most recent). Both cycle back to the full branch at the ends, and working tree changes always stay layered on top.

While filtering, a **Commits** section appears at the top of the file-list pane listing the branch's commits (newest first) so you can see what's in scope: in-scope commits are highlighted with a `●`, and excluded commits are dimmed with a `·`. At the full branch the section is hidden.

### File Review

Review any file with comments:

```sh
revise <file>
revise --output comments.md <file>
```

Opens the file in a read-only review mode — all lines shown as context, git-specific keys disabled. Add comments, then quit — comments are printed to stdout on exit. Use `--output <file>` to write comments to a file instead.

Source is syntax-highlighted by file type. Markdown gets bold headings, inline `**bold**`/`` `code` ``, and fenced code blocks highlighted by their declared language (e.g. ` ```go `).

Comments are also output on exit in normal git diff mode.

### Subcommands

| Command | Description |
|---------|-------------|
| `revise <file>` | Review a file with comments |
| `revise setup-cache` | Enable git's `core.untrackedCache` for faster refreshes |
| `revise styles` | Show file status color matrix for all staging states |
| `revise update [--pre]` | Update to the latest version |

### Features

- **Diff review** -- split-pane file list and diff viewer with color-coded diffs
- **Inline comments** -- add comments on any diff line, export for AI feedback
- **Hunk staging** -- stage, unstage, or discard individual hunks or entire files
- **File status colors** -- status indicators change color based on staging state (dim=branch, yellow=unstaged, green=staged, cyan=partial)
- **Mouse support** -- click to select files, scroll to navigate
- **NO_COLOR** -- respects the [`NO_COLOR`](https://no-color.org) environment variable

## Key Bindings

Press `?` inside revise to see the help overlay.

### General

| Key | Action |
|-----|--------|
| `←` (`h`) | Focus file list |
| `→` (`l`) | Focus diff view |
| `n` / `N` | Next / prev file (or search match while searching) |
| `Tab` / `Shift+Tab` | Cycle diff mode |
| `<` / `>` | Fewer / more commits (Branch mode) |
| `f` | Toggle fullscreen diff |
| `Esc` | Back to file list |
| `?` | Toggle help |
| `q` | Quit |

### File List

| Key | Action |
|-----|--------|
| `j` / `k`, `↑` / `↓` | Navigate files |
| `Enter` | Select file and focus diff |
| `s` | Stage file |
| `u` | Unstage file |
| `D` | Discard file |

### Diff View

| Key | Action |
|-----|--------|
| `j` / `k`, `↑` / `↓` | Move cursor |
| `}` / `{` (`]` / `[`) | Next / prev blank line |
| `/` | Search the diff (highlights matches) |
| `n` / `N` | Next / prev search match (while searching) |
| `+` / `-` | More / fewer context lines (by 3; by 1 below 3) |
| `0` | Reset context lines to the default |
| `w` | Toggle hide whitespace |
| `W` | Toggle soft line wrap |
| `g` / `G` | Top / bottom |
| `Fn+↓` / `Fn+↑` | Page down / up |
| `Enter` / `c` | Add/edit comment on line |
| `d` | Delete comment on line |
| `s` / `S` | Stage hunk / file |
| `u` / `U` | Unstage hunk / file |
| `D` | Discard hunk |

### Global

| Key | Action |
|-----|--------|
| `e` | Export comments to clipboard |
| `E` | Open current file at cursor line in `$VISUAL`/`$EDITOR` (fallback: `vi`) |
| `!` | Report issue on GitHub |

## Inspiration

- [difit](https://github.com/yoshiko-pg/difit) -- browser-based local diff viewer with comment/copy-prompt features
- [gitui](https://github.com/extrawurst/gitui) -- fast terminal UI for git

## License

[MIT](LICENSE)

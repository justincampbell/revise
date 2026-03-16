# revise

A terminal UI for reviewing local git changes and sending feedback to [Claude Code](https://docs.anthropic.com/en/docs/claude-code).

## Requirements

- Go 1.25+
- git

## Installation

```sh
go install github.com/justincampbell/revise@latest
```

## Usage

Run from within a git repository:

```sh
revise
```

Shows a split-pane view with a file list on the left and diff on the right.

- **Feature branch**: shows all changes vs merge base (as if opening a PR)
- **Default branch**: shows staged, unstaged, and untracked changes

### Diff Modes

Cycle through modes with `Tab` / `Shift+Tab`:

| Mode | What it shows |
|------|--------------|
| **Branch** | committed + staged + unstaged + untracked (broadest, default on feature branches) |
| **Staged** | staged + unstaged + untracked |
| **Unstaged** | unstaged + untracked only (narrowest) |

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `←` / `h` | Focus file list |
| `→` / `l` | Focus diff view |
| `j` / `k`, `↑` / `↓` | Navigate / scroll |
| `Enter` | Select file / open comment input |
| `n` / `N` | Next / prev file |
| `g` / `G` | Top / bottom of diff |
| `}` / `{` | Next / prev hunk |
| `f` | Toggle fullscreen diff |
| `Tab` / `Shift+Tab` | Cycle diff mode |
| `+` / `-` | More / fewer context lines |
| `c` / `Enter` | Add/edit comment on diff line |
| `d` | Delete comment |
| `e` | Export comments |
| `s` / `S` | Stage hunk / file |
| `u` / `U` | Unstage hunk / file |
| `D` | Discard hunk / file (with confirmation) |
| `Esc` | Back to file list |
| `?` | Toggle help |
| `q` / `Ctrl+C` | Quit |

### Features

- **Diff review** — split-pane file list + diff viewer with syntax-highlighted diffs
- **Comments** — add inline comments on diff lines, export for AI feedback
- **Staging** — stage/unstage/discard individual hunks or entire files
- **MCP integration** — embedded MCP server for Claude Code review loops
- **Mouse support** — click to select files, scroll to navigate
- **NO_COLOR** — respects the `NO_COLOR` environment variable

## Development

```sh
make build   # build binary
make test    # run tests
make install # go install
make clean   # remove binary
```

## Inspiration

- [difit](https://github.com/yoshiko-pg/difit) — browser-based local diff viewer with comment/copy-prompt features
- [gitui](https://github.com/extrawurst/gitui) — fast terminal UI for git

## License

[MIT](LICENSE)

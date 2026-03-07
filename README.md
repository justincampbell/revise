# revise

A terminal UI for reviewing local git changes and sending feedback to Claude Code.

## Installation

```sh
make install
```

## Usage

```sh
revise
```

Run from within a git repository. Shows a split-pane view with a file list on the left and diff on the right.

- On a feature branch: shows changes vs merge base (as if opening a PR)
- On the default branch: shows staged, unstaged, and untracked changes

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `←` / `h` | Focus file list |
| `→` / `l` | Focus diff view |
| `j` / `k`, `↑` / `↓` | Navigate / scroll |
| `Enter` | Select file and focus diff |
| `n` / `N` | Next / prev file |
| `g` / `G` | Top / bottom of diff |
| `f` | Toggle fullscreen diff |
| `Esc` | Back to file list |
| `?` | Toggle help |
| `q` / `Ctrl+C` | Quit |

## Development

```sh
make build   # build binary
make test    # run tests
make install # go install
```

## Inspiration

- **[difit](https://github.com/yoshiko-pg/difit)** — browser-based local diff viewer with comment/copy-prompt features
  - **[local-review](https://github.com/justincampbell/agent-plugins/blob/f3825b7d8ddfc61372cfbd6b3f8e939a0cfdf9cc/skills/local-review/SKILL.md) - Claude Code skill using difit
- **[gitui](https://github.com/extrawurst/gitui)** — fast terminal UI for git

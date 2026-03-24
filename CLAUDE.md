# revise

A terminal UI for reviewing local git changes and sending feedback to Claude Code.

## Project Overview

`revise` is a Go TUI application built with [Bubbletea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss). It provides a split-pane interface for reviewing git diffs: a file list on the left and a diff viewer on the right.

## Tech Stack

- **Language**: Go
- **TUI framework**: `github.com/charmbracelet/bubbletea` (Elm architecture)
- **Styling**: `github.com/charmbracelet/lipgloss`
- **Mouse support**: `tea.WithMouseCellMotion()` enabled in main

## Maintaining This File

Keep CLAUDE.md current as the project evolves. Update it when:
- A key decision is made or reversed (diff strategy, UI layout, styling rules)
- A new pattern or convention is established (testing approach, helper APIs)
- A bug is fixed that reveals a non-obvious constraint (e.g. bubbletea Esc → `"esc"`, `parseFilePath` noprefix handling)
- New files or packages are added to the architecture
- A shortcut is added, removed, or changes behavior

Do not add speculative or aspirational content — only document what is true now. Planned features belong in GitHub issues, not here.

## Development Approach

Follow TDD for all logic changes: write a failing test first, then implement. This applies to:
- Git operations (`internal/git/`)
- Model/input routing logic (`internal/ui/model.go`)
- Scroll and navigation behavior
- String utilities (`padRight`, `truncate`, `parseFilePath`, etc.)
- Key handling

UI rendering is excluded from TDD — Lipgloss layout, visual styling, and the help overlay appearance require a terminal to verify and should be tested manually.

### Testing git operations

Git functions shell out via `exec.Command` — do not replace with go-git. Tests use a `TestRepo` helper (defined in `internal/git/testhelper_test.go`) that creates real git repos in `t.TempDir()`.

Key rules:
- `NewTestRepo` sets the default branch to `main` via `git symbolic-ref` (portable across all git versions) and configures local `user.email`/`user.name` so commits work without global git config
- Call `r.Chdir()` at the start of each test — git functions use the process working directory implicitly
- Do **not** use `t.Parallel()` in tests that call `Chdir()` — `os.Chdir` is process-global
- Without a remote, `DefaultBranch()` falls back to `"main"` — name the default branch `main` in test repos and this works automatically
- For `DefaultBranch()` detection tests specifically, create a bare repo and add it as `origin`

`TestRepo` API:
```go
NewTestRepo(t)               // git init + config
r.WriteFile(path, content)   // write file to working tree
r.RemoveFile(path)           // delete file
r.Add(paths...)              // git add
r.Commit(message)            // git commit
r.CheckoutNewBranch(branch)  // git checkout -b
r.Chdir()                    // os.Chdir to repo, restored in t.Cleanup
r.StagedDiffRaw()            // git diff --cached
r.UnstagedDiffRaw()          // git diff
r.StatusPorcelain()          // git status --porcelain
```

## Build & Run

```sh
make build    # build binary
make install  # go install
make test     # go test ./...
make clean    # remove binary
```

## Architecture

```
main.go                        # entry point, flag parsing, program setup
internal/
  git/
    types.go                   # Diff, FileDiff, Hunk, Line types
    diff.go                    # GetDiff, branch detection, merge-base logic
    apply.go                   # stage/unstage/discard hunks via git apply (with lock retry)
    retry.go                   # retryOnLock helper for index.lock contention
    parse.go                   # unified diff parser
    untracked.go               # untracked file detection
    parse_test.go              # parser unit tests
    apply_test.go              # stage/unstage/discard integration tests
    diff_test.go               # mergeFileDiffs unit tests
    diff_integration_test.go   # GetDiff/StagedDiff/etc integration tests (real git repos)
    testhelper_test.go         # TestRepo helper for integration tests
  comments/
    store.go                   # JSON persistence for comments (Save/Load/StorePath)
    store_test.go              # persistence unit tests
  update/
    update.go                  # self-update via go-selfupdate (CheckForUpdate, ApplyUpdate)
    update_test.go             # integration tests (skipped in short mode)
  ui/
    model.go                   # root Bubbletea model, layout, input routing
    filelist.go                # left panel: file list
    diffview.go                # right panel: diff renderer + scroll
    keys.go                    # keyboard binding definitions (single source of truth)
    styles.go                  # all Lipgloss styles, NO_COLOR support
    comment.go                 # comment types, serialization for persistence
    comment_test.go            # comment encode/decode tests
    help.go                    # help overlay
    model_test.go              # model key routing tests
    filelist_test.go           # navigation, truncate, and tree view tests
    diffview_test.go           # scroll logic and gutter format tests
    border_snapshot_test.go    # golden tests for pane border composition
    help_test.go               # padRight unicode tests
```

## Key Decisions

### Diff Strategy

Three diff components displayed left-to-right: **Branch · Staged · Unstaged**. Tab/Shift+Tab cycles through modes. Broader modes light more components; `ModeStagedOnly` and `ModeUnstaged` each light only their respective component.

| Mode | Slider | What it shows |
|------|--------|--------------|
| **ModeBranch** | **Branch+Staged+Unstaged** | committed + staged + unstaged + untracked (broadest, default) — skipped on default branch |
| **ModeStaged** | Branch **Staged+Unstaged** | staged + unstaged + untracked |
| **ModeStagedOnly** | Branch **Staged** Unstaged | staged only (no unstaged or untracked) |
| **ModeUnstaged** | Branch Staged **Unstaged** | unstaged + untracked only (narrowest) |

- Default mode: ModeBranch on feature branches (broadest), ModeStaged on default branch
- Default branch is auto-detected via `origin/main`, `origin/master`, then `git symbolic-ref refs/remotes/origin/HEAD`
- Working tree changes for the same path replace branch diff entries (latest state wins)
- `IsOnDefaultBranch()` checks if merge-base == HEAD; re-evaluated on every diff refresh so Branch mode appears dynamically after `git checkout -b`
- Per-mode git functions: `BranchDiff()`, `WorkingTreeDiff()`, `StagedOnlyDiff()`, `UnstagedOnlyDiff()`
- Untracked files are merged into "Unstaged" — no separate untracked mode

### UI Layout

- Fixed file list width: 30 chars (or `width/3` on narrow terminals < 80 cols)
- Panel height: terminal height minus 2 rows for status area
- Fullscreen mode hides file list, expands diff to full width

### Styling

- Color-coded diff lines: green/dark-green-bg for additions, red/dark-red-bg for removals
- Bold gutter with line numbers (old/new), width 11
- Respects `NO_COLOR` env var — strips all colors when set
- Focused panel gets cyan border; unfocused gets dim border

### Keyboard Bindings

All bindings are defined in `internal/ui/keys.go` (`allBindings`) and used to generate both the in-app help overlay (`?`) and the `--help` CLI output. This is the single source of truth — do not define bindings elsewhere.

Bubbletea maps the Escape key to the string `"esc"` (not `"escape"`) — use `case "esc":` in switch statements.

| Key | Action |
|-----|--------|
| `←` (`h`) | Focus file list |
| `→` (`l`) | Focus diff view |
| `n` / `N` | Next / prev file |
| `Tab` / `Shift+Tab` | Cycle diff mode |
| `+`/`-` | More/fewer context lines |
| `t` | Toggle tree view in file list |
| `f` | Toggle fullscreen diff |
| `Esc` | Back to file list (exits fullscreen if needed) |
| `?` | Toggle help overlay |
| `q` / `Ctrl+C` | Quit |
| `e` | Export comments (works from any panel) |
| `j/k`, `↑/↓` | Navigate files (file list) / move cursor (diff) |
| `Enter` | Select file and focus diff (file list) / open comment input (diff) |
| `}` / `{` (`]`/`[`) | Next / prev hunk |
| `g` / `G` | Top / bottom of diff |
| `Fn+↓` / `Space` | Page down |
| `Fn+↑` | Page up |
| `c` | Add/edit file comment (file list) / Add/edit comment on line (diff) |
| `d` | Delete file comment (file list) / Delete comment on line (diff) |
| `C` | Clear all comments (with confirmation) |
| `s` / `S` | Stage hunk / file (file list: `s` stages file) |
| `u` / `U` | Unstage hunk / file (file list: `u` unstages file) |
| `w` | Toggle hide whitespace changes |
| `D` | Discard hunk / file (with `y` confirmation) |
| `Ctrl+U` | Apply available update |

### File List Tree View

- Toggle with `t` key (works from either panel)
- Groups files by directory with indentation; directories shown as `dir/` with dim style
- Cursor navigates files only (directory headers are skipped)
- `buildTreeRows()` creates a flat list of `treeRow` structs from files, emitting directory headers when the directory path changes
- Tree state (`treeView` bool) is preserved across diff refreshes
- Mouse clicks on directory headers are ignored; clicks on file rows select the file
- Default is flat view (tree off)

### Mouse Support

- Scroll wheel up/down: scrolls whichever panel the cursor is over
- Left click on file list: selects the file at that row (accounts for border + header offset)
- Panel detection uses X position relative to file list width

### Comment Persistence

Comments are persisted to `os.TempDir()/revise/<hash>.json` where hash = `sha256(repoRoot + ":" + branchName)[:16]`. Auto-saved on every add/edit/delete, loaded on startup. The `internal/comments/` package handles storage; `internal/ui/comment.go` handles serialization between the internal `commentKey` struct and string-keyed JSON.

File-level comments use `lineNum: 0` in the `commentKey` struct (diff line numbers are always >= 1). They are displayed at the top of the diff view before hunks, and exported as "File comment: ..." in the export format.

### Diff Panel Footer

The diff panel bottom border shows three overlaid elements:
- **Left**: "Context: N" (dim when default 3, cyan when changed)
- **Center**: "+A/-R" change counts with slash anchored to pane center
- **Right**: "Whitespace" (dim) or "Whitespace hidden" (cyan when `w` toggled)

### Diff Parsing

- `parseFilePath` handles both standard (`diff --git a/foo b/foo`) and no-prefix (`diff --git foo foo`) formats — the latter occurs when `diff.noprefix=true` is set in the user's git config
- `padRight` and all column-width calculations use rune count (`len([]rune(s))`), not byte length, to handle multi-byte Unicode characters like arrow symbols correctly

## File Status Indicators

| Symbol | Meaning |
|--------|---------|
| `M` | Modified |
| `A` | Added |
| `D` | Deleted |
| `R` | Renamed |
| `?` | Untracked |


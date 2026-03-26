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

Do not add speculative or aspirational content — only document what is true now. Planned features belong in GitHub issues, not here.

## Workflow

Work is tracked in GitHub issues. When asked, pick up issues one at a time (or in small related groups). Every change goes through a PR:

1. Create a feature branch from `main`
2. Implement the change with tests
3. Run `make install` so the user can manually test
4. Open the PR (draft if not ready for review)
5. When asked to merge, wait for CI to pass first
6. After merge, pull main and rebase any remaining branches

When adding or changing user-facing features, check whether these need updating:
- **`keys.go`** — keyboard binding definitions (single source of truth for help + CLI)
- **Help overlay** — generated from `keys.go`, but verify grouping/descriptions
- **CLAUDE.md** — architecture map, key decisions
- **README.md** — if the feature is user-facing or changes install/usage
- **CHANGELOG.md** — add an entry for the change

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
    apply.go                   # stage/unstage/discard hunks via git apply
    parse.go                   # unified diff parser
    format.go                  # plain-text diff formatter (revise diff subcommand)
    untracked.go               # untracked file detection
    parse_test.go              # parser unit tests
    apply_test.go              # stage/unstage/discard integration tests
    diff_test.go               # mergeFileDiffs unit tests
    format_test.go             # formatter tests
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
    filelist_test.go           # navigation and truncate tests
    diffview_test.go           # scroll logic and gutter format tests
    border_snapshot_test.go    # golden tests for pane border composition
    help_test.go               # help overlay, padRight, pluralize tests
    styles_demo.go             # --styles-demo output for visual testing
```

## Key Decisions

### Diff Strategy

Four modes: ModeBranch (broadest), ModeStaged, ModeStagedOnly, ModeUnstaged (narrowest). Tab/Shift+Tab cycles.

- Default mode: ModeBranch on feature branches (broadest), ModeStaged on default branch
- Default branch is auto-detected via `origin/main`, `origin/master`, then `git symbolic-ref refs/remotes/origin/HEAD`
- Working tree changes for the same path replace branch diff entries (latest state wins)
- `IsOnDefaultBranch()` checks if merge-base == HEAD; re-evaluated on every diff refresh so Branch mode appears dynamically after `git checkout -b`
- Per-mode git functions: `BranchDiff()`, `WorkingTreeDiff()`, `StagedOnlyDiff()`, `UnstagedOnlyDiff()`
- Untracked files are merged into "Unstaged" — no separate untracked mode

### UI Layout

- Fixed file list width: 30 chars (or `width/3` on narrow terminals < 80 cols)
- Panel height: terminal height minus 3 rows (status bar + mode slider)
- Fullscreen mode hides file list, expands diff to full width

### Styling

- Color-coded diff lines: green/dark-green-bg for additions, red/dark-red-bg for removals
- Bold gutter with line numbers (old/new), width 11
- Respects `NO_COLOR` env var — strips all colors when set
- Focused panel gets cyan border; unfocused gets dim border
- Help overlay uses yellow border to distinguish from panels
- Lipgloss `Width(n)`/`Height(n)` include padding but exclude borders — when setting explicit dimensions, add padding to the content size

### File Review Mode

`revise <file>` opens any file in a read-only review mode with full comment support. All lines are shown as context (no diff coloring). Starts fullscreen with diff view focused. Git-specific keys (stage/unstage/discard, mode cycling, context lines, whitespace toggle) are disabled — guarded by a single `fileReviewMode` check per key group in the Update handlers. The help overlay filters bindings via `FileReviewBindingGroups()` which uses the `GitOnly` flag on `Binding`.

Comments are output to stdout on exit (both in file review and git diff modes), enabling integration with Claude Code: Claude launches `revise`, user reviews and comments, comments print on close.

### Keyboard Bindings

All bindings are defined in `internal/ui/keys.go` (`allBindings`) and used to generate both the in-app help overlay (`?`) and the `--help` CLI output. This is the single source of truth — do not define bindings elsewhere. Each `Binding` has a `GitOnly` flag — when true, the binding is hidden in file review mode help and disabled in input handling.

Bubbletea maps the Escape key to the string `"esc"` (not `"escape"`) — use `case "esc":` in switch statements.

### Mouse Support

- Scroll wheel up/down: scrolls whichever panel the cursor is over (help overlay intercepts scroll when visible)
- Left click on file list: selects the file at that row (accounts for border + header offset)
- Panel detection uses X position relative to file list width

### Comment Persistence

Comments are persisted to `os.TempDir()/revise/<hash>.json` where hash = `sha256(repoRoot + ":" + branchName)[:16]`. Auto-saved on every add/edit/delete, loaded on startup. The `internal/comments/` package handles storage; `internal/ui/comment.go` handles serialization between the internal `commentKey` struct and string-keyed JSON.

File-level comments use `lineNum: 0` in the `commentKey` struct (diff line numbers are always >= 1). They are displayed at the top of the diff view before hunks, and exported as "File comment: ..." in the export format.

### Diff Parsing

- `parseFilePath` handles both standard (`diff --git a/foo b/foo`) and no-prefix (`diff --git foo foo`) formats — the latter occurs when `diff.noprefix=true` is set in the user's git config
- `padRight` and all column-width calculations use rune count (`len([]rune(s))`), not byte length, to handle multi-byte Unicode characters like arrow symbols correctly



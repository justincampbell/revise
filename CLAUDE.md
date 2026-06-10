# revise

A terminal UI for reviewing local git changes and sending feedback to Claude Code.

## Project Overview

`revise` is a Go TUI application built with [Bubbletea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss). It provides a split-pane interface for reviewing git diffs: a file list on the left and a diff viewer on the right.

## Tech Stack

- **Language**: Go
- **TUI framework**: `charm.land/bubbletea/v2` (Elm architecture)
- **Styling**: `charm.land/lipgloss/v2`
- **Terminal modes**: in Bubble Tea v2, alt screen / mouse / focus reporting are set declaratively on the `tea.View` returned by `Model.View()` (`AltScreen`, `MouseMode`, `ReportFocus`), not via program options. `View()` returns `tea.View` (wrap a string with `tea.NewView`), not a plain `string`.

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

## Formatting

- ALWAYS format Go code you write with `goimports`. If `goimports` is not available, use `gofmt`.
- Run `make lint` to check for lint issues before committing.

## Build & Run

```sh
make           # lint + test (default)
make build     # build revise + revise@VERSION
make install   # install revise + revise@VERSION to GOBIN
make test      # go test ./...
make lint      # go.mod tidy check + golangci-lint
make clean     # remove binaries
```

`make build` and `make install` produce two binaries: `revise` (for normal use) and `revise@VERSION` (e.g. `revise@v0.2.0` on a tag, `revise@v0.2.0-gb18a73d` on a dev build). The versioned copy lets you keep and compare specific builds side-by-side.

## Architecture

```
main.go                        # entry point, flag parsing, program setup
internal/
  git/
    types.go                   # Diff, FileDiff, Hunk, Line types
    diff.go                    # GetDiff, branch detection, merge-base logic, git commands
    merge.go                   # diff composition: mergeFileDiffs, tagHunks, composeBranch/WorkingTree
    apply.go                   # stage/unstage/discard hunks via git apply
    parse.go                   # unified diff parser
    format.go                  # plain-text diff formatter (revise diff subcommand)
    untracked.go               # untracked file detection
    merge_test.go              # mergeFileDiffs/compose unit tests
    merge_bench_test.go        # merge performance benchmarks
    parse_test.go              # parser unit tests
    apply_test.go              # stage/unstage/discard integration tests
    format_test.go             # formatter tests
    diff_integration_test.go   # GetDiff/StagedDiff/etc integration tests (real git repos)
    testhelper_test.go         # TestRepo helper for integration tests
  refresh/
    policy.go                  # adaptive cadence policy (NextDelay, Debounce); pure logic
    policy_test.go             # table-driven unit tests
  fswatch/
    watcher.go                 # fsnotify wrapper: watches tracked dirs + git dir, coalesces bursts
    watcher_test.go            # integration tests using real git tempdirs
  comments/
    store.go                   # JSON persistence for comments (Save/Load/StorePath)
    store_test.go              # persistence unit tests
  devwatch/
    devwatch.go                # poll a binary for changes, fire callback on replacement (--dev flag)
    devwatch_test.go           # watcher unit tests
  editor/
    editor.go                  # build $VISUAL/$EDITOR command with file+line args (E key)
    editor_test.go             # unit tests for editor command construction
  update/
    update.go                  # self-update via go-selfupdate (CheckForUpdate, ApplyUpdate)
    update_test.go             # integration tests (skipped in short mode)
  ui/
    model.go                   # root Bubbletea model, layout, input routing
    filelist.go                # left panel: file list
    diffview.go                # right panel: diff renderer + scroll
    keys.go                    # keyboard binding definitions (single source of truth)
    styles.go                  # all Lipgloss styles, NO_COLOR support
    theme.go                   # color palettes + chroma style entries per theme
    syntax.go                  # chroma syntax highlighting, indent guides, Markdown fences
    syntax_test.go             # highlight, fence detection, indent guide tests
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
- `theme.go` resolves a `themeColors` palette per theme. The `auto`/`auto-daltonized` themes consult `isDark` (detected via `lipgloss.HasDarkBackground`): on a dark terminal they use ANSI named colors to adopt the terminal's palette, but on a light terminal those (notably white/yellow) are illegible, so `autoPalette`/`autoDaltonizedPalette` mirror the curated `github-light` palettes. Diff-line backgrounds are baked into syntax-highlight tokens, so a dark `addedBg`/`removedBg` on a light terminal makes the dark highlight text unreadable — this is why the palette must follow `isDark` (#198). `ThemeAutoDaltonized` therefore also appears in `chromaStyleEntries` (it only reaches there on a light terminal; dark selects the `native` chroma style first in `highlightLine`).

### Syntax Highlighting

`internal/ui/syntax.go` highlights each line via chroma (`highlightLine`), keyed by `lexerFor(filePath)`. Highlighting is **line-by-line** so it composes with per-line diff backgrounds and the per-line cache — the trade-off is that multi-line lexer state (block comments, raw strings) isn't tracked, which is acceptable and consistent for all languages. Per-theme token colors live in `theme.go` (`chromaStyleEntries`); the custom `chromaFormatter` bakes the diff background into every token so ANSI resets don't clear it mid-line.

Markdown gets two extras (#194):
- **Headings**: chroma only emits `GenericHeading`/`GenericSubheading` when the line ends in a newline, so `highlightLine` appends `\n` before tokenising (the formatter trims and skips the resulting empty token). The formatter force-bolds heading tokens so they stand out even under named chroma styles (e.g. "github") that don't bold them; custom themes also give headings a color in `theme.go`.
- **Fenced code blocks**: `buildLines` runs a per-hunk fence state machine (`codeFenceLang` detects ` ``` `/`~~~` open/close + info-string language). Lines inside a fence are highlighted with the declared language via the `langOverride` param threaded through `renderDiffLine` → `highlightLine` (`lexerByName`, case-insensitive, alias-aware). `langOverride` is part of the highlight cache key. Fence state resets per hunk since a diff's hidden gaps make cross-hunk tracking unreliable. Only active for Markdown files (`isMarkdownFile`).

### File Review Mode

`revise <file>` opens any file in a read-only review mode with full comment support. All lines are shown as context (no diff coloring). Starts fullscreen with diff view focused. Git-specific keys (stage/unstage/discard, mode cycling, context lines, whitespace toggle) are disabled — guarded by a single `fileReviewMode` check per key group in the Update handlers. The help overlay filters bindings via `FileReviewBindingGroups()` which uses the `GitOnly` flag on `Binding`.

Navigation note (applies to all modes, not just file review): `}`/`{` (`]`/`[`) jump to the next/previous blank line (Vim-style paragraph boundary) via `nextParagraph`/`prevParagraph`. There is no dedicated hunk-to-hunk navigation — in a diff the nearest blank line usually sits at the next change, so paragraph jumping doubles as change-to-change navigation and stays consistent across modes (#122). Blank lines are flagged at build time as `lineRef.isBlank`. (`hunkStarts`/`currentHunkIndex` remain, used by hunk stage/unstage/discard.)

Comments are output to stdout on exit (both in file review and git diff modes), enabling integration with Claude Code: Claude launches `revise`, user reviews and comments, comments print on close.

### Keyboard Bindings

All bindings are defined in `internal/ui/keys.go` (`allBindings`) and used to generate both the in-app help overlay (`?`) and the `--help` CLI output. This is the single source of truth — do not define bindings elsewhere. Each `Binding` has a `GitOnly` flag — when true, the binding is hidden in file review mode help and disabled in input handling.

Bubbletea maps the Escape key to the string `"esc"` (not `"escape"`) — use `case "esc":` in switch statements.

### Mouse Support

- Scroll wheel up/down: scrolls whichever panel the cursor is over (help overlay intercepts scroll when visible)
- Left click on file list: selects the file at that row (accounts for border + header offset)
- Panel detection uses X position relative to file list width
- Bubble Tea v2 splits the old `MouseMsg` into per-action types: wheel events arrive as `tea.MouseWheelMsg`, a button press as `tea.MouseClickMsg` (no `Action` check needed — a click *is* a press), release as `tea.MouseReleaseMsg` (unhandled, so releases are ignored). Coordinates/button come from `msg.Mouse()` (`mouseFocusDiff` takes a `tea.Mouse`).

### Theme Detection (light/dark)

The initial light/dark choice for auto themes is detected before the program starts via `lipgloss.HasDarkBackground` in `main.go`. To track OS appearance changes *while running*, `Model.Init` enables DEC mode 2031 (`tea.Raw(ansi.SetModeLightDark)`) when an auto theme is active; the terminal then reports scheme changes as `uv.LightColorSchemeEvent`/`uv.DarkColorSchemeEvent` (`uv` = `github.com/charmbracelet/ultraviolet`, the v2 input layer; `tea.Msg = uv.Event`). `Update` routes these through `applyColorScheme`, which re-runs `SetTheme` and rebuilds the diff lines — but only for auto themes, and only when the scheme actually flipped. Explicit themes are left untouched (#198). Works in file review mode too.

### Comment Persistence

Comments are persisted to `os.TempDir()/revise/<hash>.json` where hash = `sha256(repoRoot + ":" + branchName)[:16]`. Auto-saved on every add/edit/delete, loaded on startup. The `internal/comments/` package handles storage; `internal/ui/comment.go` handles serialization between the internal `commentKey` struct and string-keyed JSON.

File-level comments use `lineNum: 0` in the `commentKey` struct (diff line numbers are always >= 1). They are displayed at the top of the diff view before hunks, and exported as "File comment: ..." in the export format.

### Auto-refresh

Three pieces, each with one job:

- **`internal/refresh/`** — pure cadence policy. `NextDelay(lastDuration)` clamps `multiplier × lastDuration` to `[Min, Max]` so cadence self-throttles when `git status` is slow (10-20 revise instances on worktrees of one repo cause I/O contention; see #129). `Debounce(lastStart, now)` returns the wait until the next refresh request can fire.
- **`internal/fswatch/`** — fsnotify wrapper. Watches the parent directories of tracked files (via `git ls-files`, so it's gitignore-correct without walking node_modules) plus the git directory itself (filtered to `index`/`HEAD`). Coalesces event bursts into a single emitted `Event`.
- **`internal/ui/model.go`** — wires them together. `requestRefresh()` is the single funnel for FocusMsg, fsEventMsg, and scheduled ticks. It bumps a `pollGen` counter on every request so any earlier in-flight `tea.Tick` becomes stale and is dropped (eliminates the "scheduled at +25s, focus at +24.9s" dead zone).

The fs watcher is attached via `Model.WithFSWatcher(w)`. If `fswatch.New` fails (network FS, FD limits) or `--no-watch` / `REVISE_NO_WATCH=1` is set, the model gets no watcher and auto-refresh runs on a timer only.

### Diff Parsing

- `parseFilePath` handles both standard (`diff --git a/foo b/foo`) and no-prefix (`diff --git foo foo`) formats — the latter occurs when `diff.noprefix=true` is set in the user's git config
- `padRight` and all column-width calculations use rune count (`len([]rune(s))`), not byte length, to handle multi-byte Unicode characters like arrow symbols correctly

### Incremental Search

`/` opens a search box (rendered in the status bar, not as an overlay) for incremental search in the diff view. Design (#72):

- **Scope**: the current file's diff only. `setFile` clears the search, so switching files resets it. Cross-file search was deliberately left out to keep `n`/`N` unambiguous.
- **Case-insensitive**, rune-safe matching via `matchRanges` (returns `[start,end)` rune-index ranges) and `lineContainsQuery`.
- **Highlighting happens at build time but preserves syntax colors**: `buildLines` passes `m.searchQuery` to `renderDiffLine`, which first renders the line normally (syntax highlight or base diff-line style), then calls `overlaySearchHighlight` to re-style *only the matched columns* with `searchMatchStyle`. The overlay slices the already-styled string by visual column (`clipCols` → `ansi.TruncateLeft`/`ansi.Truncate`, then `ansi.Strip` + restyle the matched slice), so the rest of the line keeps its colors. `matchColumnRanges` converts rune-index matches to visual columns (wide-rune-safe). `setSearch` calls `buildLines` on every keystroke; matches don't add/remove lines, so cursor indices stay stable.
- **Match list**: `searchMatches` holds navigable line indices containing the query; `searchIdx` is the current position (-1 when none). `computeSearchMatches` scans `lineRef.content` (plain source text, stored at build time).
- **Navigation**: `nextMatch`/`prevMatch` wrap. After moving, `ensureMatchVisible` scrolls horizontally (when wrap is off) so an off-screen match is brought into the viewport. `n`/`N` in `model.go` route to these when `searchQuery != ""`, otherwise to next/prev file (the default). While the search box is open, all keys (including `n`/`N`) feed the query.
- **Lifecycle**: `/` → `startSearch` (records `searchOrigin = cursor`, focuses the diff view). Typing → `setSearch` (incremental: rebuild, recompute, jump to first match at/after `searchOrigin`, wrapping). `Enter` commits (box closes, query/matches retained, status bar shows `current/total` + nav hint). `Esc` clears — both inside the box and after committing (a guard before the main key switch in `Update` intercepts `esc` when `searchQuery != ""`).

### Soft Line Wrap

`W` (or `Alt+Z`) toggles `diffViewModel.wrapEnabled`. The key design choice: `cursor` and `offset` stay **logical-line** indices into `lines[]` — wrap only changes how lines map to display rows. This keeps all navigation (hunks, marks, comments) working unchanged.

The display-row math lives in a few primitives in `diffview.go`:
- `displayRows(idx, avail, hOffset)` — the single source of truth for how a logical line renders: one row (truncated) when wrap is off, multiple word-wrapped rows when on. Continuation rows are indented by the gutter width (6) so content aligns; wrapping uses `ansi.Wrap` after splitting gutter from content so styling is preserved.
- `lineRows`/`rowSpan`/`lineAtRow` — derive row counts and map display-row offsets back to logical lines.
- `ensureCursorVisible`/`bottomOffset`/`lastVisibleLine` — replace the old inline `offset = cursor - viewHeight + 1` arithmetic everywhere (moveCursor, hunk jumps, rebuild, scroll clamp). When wrap is off they reduce to the original behavior, so existing tests still hold.

`clickToAbsIdx` and `scrollForCommentInput` go through the same primitives, so mouse clicks and the inline comment box stay correct with wrap on. Horizontal scroll is disabled (hOffset forced to 0) while wrapping.

### Scrollbar

When diff-view content overflows the viewport, a scrollbar thumb is drawn on the right border (suppressed when everything fits, so short files stay clean and existing border snapshots are unaffected). Design (#196):

- `overlayScrollbar` replaces the rightmost column (the `│` border char) of the thumb's track rows with a solid `█` block, after all border-title/footer decoration is done (those only touch the top/bottom lines, never the interior right border). A full block — rather than a heavy line like `┃`, which some fonts render indistinguishably from the `│` track — makes the thumb stand out. It's `Bold` and colored `colorCyan` when the pane is focused, else `colorScrollbarThumb` (`p.dim`, a mid gray brighter than the border).
- Geometry comes from `scrollMetrics()` (total display rows, rows above the viewport, viewport height). Row-based (not logical-line-based) math keeps it correct with soft wrap on. `scrollbarThumb` returns `(top, size, show)` with `size ≈ viewH²/totalRows` clamped to `[1, viewH-1]`; `top` is offset-based so the thumb tracks the viewport, not the cursor.



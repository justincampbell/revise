# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.5.2] - 2026-06-10

### Added

- The diff view now shows a scrollbar on the right border when content overflows the viewport: the thumb's size reflects how much of the file is visible and its position shows where you are. It appears only when there's something to scroll, so short files stay uncluttered. Useful for keeping your bearings when reviewing long files, e.g. plans via `revise plan.md`. Works in both git diff and file review mode (#196)

## [0.5.1] - 2026-06-09

### Added

- Markdown files now render headings in bold and syntax-highlight fenced code blocks using their declared language (e.g. ` ```go `), in addition to the inline `**bold**`/`` `code` `` formatting that already worked. Code inside a fence is highlighted with that language's lexer; bare (language-less) fences stay plain. Works in both git diff and file review mode (#194)

## [0.5.0] - 2026-06-05

### Added

- `/` starts an incremental search in the diff view: the matched text highlights as you type (the rest of the line keeps its syntax colors), the cursor jumps to the nearest match, and `n`/`N` step forward/backward between matches (wrapping). Off-screen matches scroll into view horizontally. The status bar shows a `current/total` count; `Esc` clears the search. Case-insensitive and scoped to the current file. Works in both git diff and file review mode — `n`/`N` keep their next/prev-file meaning when no search is active (#72)
- `W` toggles soft line wrap in the diff view, wrapping long lines onto multiple display rows at the viewport width instead of clipping them. Continuation rows are indented past the line-number gutter so content stays aligned; a `Wrap` indicator appears on the bottom border when enabled. Works in both git diff and file review mode — useful for reviewing prose-heavy planning/architecture docs (#163)
- `E` opens the current file at the cursor line in `$VISUAL`/`$EDITOR` (falls back to `vi`); the TUI suspends until the editor exits, then reloads the diff so any edits show up immediately. Recognizes vim/nvim/nano/emacs (`+N file`), VS Code/Cursor/Codium (`--goto file:N`), and Sublime/Zed/Atom (`file:N`); other editors fall back to the `+N` form (#191)

### Changed

- `}`/`{` (`]`/`[`) now jump to the next/previous blank line (Vim-style paragraph boundary) in **all** modes, instead of jumping between hunks. In a diff the nearest blank line is usually right at the next change, so this still serves as change-to-change navigation while behaving consistently with file review mode. Dedicated hunk-to-hunk navigation is removed (#122)

## [0.4.2] - 2026-05-26

### Fixed

- Auto-refresh now detects commits whose working tree state matches the pre-stage one, so mode auto-promotion to Branch fires after a rapid stage+commit on a fresh feature branch

## [0.4.1] - 2026-05-15

### Fixed

- `revise` no longer exits with `Error: repository has no commits` when run in a repo with no commits. The TUI launches normally, showing staged and untracked files with a "No commits yet" hint in the status bar (#189)

## [0.4.0] - 2026-05-08

### Added

- `--dev` flag auto-restarts the running TUI when the binary is replaced (e.g. by `make install`), preserving argv and env (#160)
- `setup-cache` subcommand to enable git's `core.untrackedCache` for faster refreshes; a startup tip surfaces in the status bar when the cache is disabled (#160)
- Horizontal scroll in the diff view via →/← (or h/l) and mouse wheel left/right (#121)
- `revise diff --mode=<branch|staged|staged-only|unstaged>` selects which diff to print non-interactively (default: auto-detect, same as launching the TUI) (#177)
- `revise diff --hunks` prints TUI-style output — file path header, `[source]` tag with function context, and a line-number gutter — instead of unified diff format (#177)
- `r` key forces a diff reload, as an escape hatch when auto-refresh has gotten stuck (e.g. fingerprint stable, polling wedged, focus reporting flaky) (#186)
- `--debug` flag shows a refresh-debug strip above the status bar with live timings: focus state, fsnotify on/off, polling status, time since the last fingerprint check / diff reload (with durations), time until the next scheduled tick, and a generation counter. Independent of `--dev`; the two compose freely (#144)

### Changed

- Right arrow no longer toggles fullscreen — it scrolls right when the diff is focused. Use `f` to toggle fullscreen. Left arrow scrolls left first, then falls back to focusing the file list when already at column 0 (#121)
- The `Branch` label in the mode slider is always shown — greyed out (faint + strikethrough) when on the default branch where Branch mode isn't available — so its position stays stable and users can see the mode exists (#148)

### Fixed

- When a fresh branch diverges from default while revise is running (e.g. after a commit), the mode auto-promotes from Staged to Branch so new commits become visible, unless the user has explicitly picked a mode (#148)
- When a commit on the default branch causes auto-promotion to ModeBranch, the diff is now reloaded under the new mode instead of leaving stale (often empty) contents on screen until the user manually refreshes (#187)
- Marked lines are now visibly distinct: brighter mark backgrounds and a colored `▌` bookmark stripe in the leading prefix column. Daltonized themes use a purple mark color so the highlight no longer collides with the blue "added" background (#173)
- Commented lines also get a yellow `▌` bookmark stripe in the leading prefix column, matching the mark-line treatment (#173)
- fswatch now installs a watch on newly-created directories on the fly, so `mkdir new-feature && touch new-feature/foo.go` triggers a refresh immediately instead of waiting for the next periodic poll (#144)
- Auto-refresh now wakes on file changes (via fsnotify) and on terminal focus, instead of waiting up to 30s for the next polling tick. Cadence is adaptive — it self-throttles based on how long the last `git status` took, so running 10-20 revise instances on shared-repo worktrees no longer piles up I/O. Pass `--no-watch` (or set `REVISE_NO_WATCH=1`) to disable the fsnotify path and fall back to timer-only refreshes (#144)
- Startup no longer runs `git update-index --test-untracked-cache`, which created `mtime-test-XXXXXX/` directories in the working tree (and left them behind if the process exited before cleanup). The startup tip now fires on any repo with `core.untrackedCache` disabled; the FS test still runs on demand inside `revise setup-cache` (#178)
- fswatch now caps fsnotify watches at 500 — refuses to install when a repo has more tracked directories at startup, and refuses runtime adds (e.g. directories created by `npm install`) once the cap is hit. Without the cap, large repos × concurrent revise instances could exhaust the system fd limit and crash startup with `too many open files in system` (#183)
- File list and diff view now always render to the same total height, so the status bar no longer "jumps up" when switching between long and short files or viewing a repo with few changes (#169)

## [0.3.0] - 2026-04-15

### Added

- Syntax highlighting via chroma (token colors layered over diff backgrounds) (#141)
- `--theme` flag with ten palettes across three color systems: `auto` (default), `auto-daltonized`, `charmtone-dark`, `charmtone-dark-daltonized`, `charmtone-light`, `charmtone-light-daltonized`, `github-dark`, `github-dark-daltonized`, `github-light`, `github-light-daltonized`
- Detect binary files and show placeholder instead of garbled content (#154)
- `make build` and `make install` produce a version-stamped copy (`revise@VERSION`) alongside `revise` for testing multiple versions side-by-side (#153)

### Changed

- Upgraded to lipgloss v2 with terminal background auto-detection
- Default theme changed from `dark` to `auto` (adapts to terminal light/dark mode)
- Lowered `go.mod` go directive from `1.25.3` to `1.24.11` (actual minimum), added `toolchain` directive (#151)
- Added `lint-gomod` check to verify `go.mod` stays tidy and go version doesn't drift (#151)

## [0.2.0] - 2026-03-30

### Added

- Line marking with `m` key to highlight lines of interest (#105)
- `--theme` flag with four color palettes: `dark` (default), `light`, `dark-daltonized`, `light-daltonized` (#136)
- File review mode: `revise <file>` opens any file for review with comments (#55, #119)
- Output comments to stdout on exit for Claude Code integration (#118)
- Show git sha in dev version output (#134)
- Auto-refresh diff every 2 seconds when files change (#87)
- Include line contents in comment export (#85)
- File-level comments with `c`/`d` on file list (#73)
- `C` key to clear all comments with confirmation (#74)
- Mode slider visible in fullscreen diff view (#76)
- GitHub issue templates (#77)
- `}` jumps past last hunk to end of diff (#78)
- Persist comments to temp file so they survive restarts (#11)
- Staged-only diff mode inserted between Staged and Unstaged in the mode cycle (#56)
- Toggle to hide whitespace-only changes with `w` key (#18)
- `!` key to report issues on GitHub (#45)
- `S`/`U` keys to stage/unstage entire file from the file list (#44)
- `Enter` as alternative to `y` for confirming discard (#43)
- Stage, unstage, and discard hunks and files with `s`/`u`/`D` keys (#2)
- Branch mode available on default branch when local is ahead of remote (#26)
- Mouse click support for the diff mode slider (#22)
- Hunk navigation with `}`/`{` keys (#25)
- Adjustable context lines with `+`/`-` keys (#29)
- Diff mode switcher with `Tab`/`Shift+Tab` cycling, mode slider in file list header (#40)
- Context and whitespace indicators in diff panel footer (#40)
- Inline comments on diff lines with `c`/`d`/`e` keys (#1)
- Fullscreen diff toggle with `f` and right arrow (#9)
- Support for non-origin remotes (#17)

### Fixed

- Mouse click under inline comment selecting wrong line (#127)
- File list cursor position lost on diff reload (#113)
- Trailing blank lines in clipboard export (#128)
- Self-update failing with "executable not found in tar" due to missing executable path (#124)
- Retry git operations on index.lock contention with exponential backoff (#46)
- Branch mode becoming unavailable after checking out a new branch while running (#57)
- Cache untracked file reads to avoid redundant I/O when adjusting context lines (#35)
- Branch diff showing remote changes for zero-commit feature branches (#49)
- Branch-added files that were later deleted no longer appear (#48)
- Diff cursor hidden when no file is selected (#23)
- Cursor landing on non-navigable lines (headers, blank lines)
- Duplicate comment display for old/new line overlap
- Click offset below inline input box

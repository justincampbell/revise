# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- `--dev` flag auto-restarts the running TUI when the binary is replaced (e.g. by `make install`), preserving argv and env (#160)
- `setup-cache` subcommand to enable git's `core.untrackedCache` for faster refreshes; a startup tip surfaces in the status bar when the cache is disabled but the filesystem supports it (#160)
- Horizontal scroll in the diff view via →/← (or h/l) and mouse wheel left/right (#121)
- `revise diff --mode=<branch|staged|staged-only|unstaged>` selects which diff to print non-interactively (default: auto-detect, same as launching the TUI). Useful for scripting and debugging (#177)
- `revise diff --hunks` prints TUI-style output — file path header, `[source]` tag with function context, and a line-number gutter — instead of unified diff format. Built on shared formatters (`FormatGutter`, `HunkHeaderText`) the TUI also uses (#177)

### Changed

- Right arrow no longer toggles fullscreen — it scrolls right when the diff is focused. Use `f` to toggle fullscreen. Left arrow scrolls left first, then falls back to focusing the file list when already at column 0 (#121)

### Changed

- The `Branch` label in the mode slider is always shown — greyed out (faint + strikethrough) when on the default branch where Branch mode isn't available — so its position stays stable and users can see the mode exists (#148)

### Fixed

- When a fresh branch diverges from default while revise is running (e.g. after a commit), the mode auto-promotes from Staged to Branch so new commits become visible, unless the user has explicitly picked a mode (#148)
- Marked lines are now visibly distinct: brighter mark backgrounds and a colored `▌` bookmark stripe in the leading prefix column. Daltonized themes use a purple mark color so the highlight no longer collides with the blue "added" background (#173)
- Commented lines also get a yellow `▌` bookmark stripe in the leading prefix column, matching the mark-line treatment (#173)
- fswatch now installs a watch on newly-created directories on the fly, so `mkdir new-feature && touch new-feature/foo.go` triggers a refresh immediately instead of waiting for the next periodic poll (#144)
- Auto-refresh now wakes on file changes (via fsnotify) and on terminal focus, instead of waiting up to 30s for the next polling tick. Cadence is adaptive — it self-throttles based on how long the last `git status` took, so running 10-20 revise instances on shared-repo worktrees no longer piles up I/O. Pass `--no-watch` (or set `REVISE_NO_WATCH=1`) to disable the fsnotify path and fall back to timer-only refreshes (#144)

### Added

- `--debug` flag shows a refresh-debug strip above the status bar with live timings: focus state, fsnotify on/off, polling status, time since the last fingerprint check / diff reload (with durations), time until the next scheduled tick, and a generation counter. Independent of `--dev`; the two compose freely (#144)

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

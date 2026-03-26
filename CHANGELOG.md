# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- `--theme` flag with four color palettes: `dark` (default), `light`, `dark-daltonized`, `light-daltonized`
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

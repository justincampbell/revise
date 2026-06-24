# revise plugin for Claude Code

A Claude Code plugin that wraps the [revise](https://github.com/justincampbell/revise) TUI: review your local changes (or any file) with inline comments, and Claude reads the comments back and acts on them.

## Install

The plugin and the `revise` binary install separately — the plugin assumes `revise` is already on your PATH.

```sh
# 1. Install the binary
brew install justincampbell/tap/revise        # or: go install github.com/justincampbell/revise@latest

# 2. Install the plugin
/plugin marketplace add justincampbell/revise
/plugin install revise@revise
```

For local development, point Claude Code at this directory instead:

```sh
claude --plugin-dir /path/to/revise/plugin
```

## Skills

- `/revise` — review the current working/branch diff, then address the comments you leave.
- `/revise <file>` — review a single file (e.g. a plan or doc), then address the comments.

In tmux, revise opens in a near-fullscreen popup that closes when you quit; Claude then reads your comments and works through them. Outside tmux, Claude asks you to run revise yourself.

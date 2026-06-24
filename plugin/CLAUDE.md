# revise plugin

Adds a `/revise` skill that opens the [revise](https://github.com/justincampbell/revise) TUI for reviewing local git changes (or a single file), then reads back the inline comments and works through them.

## Requires

The `revise` binary must be installed and on PATH:

```
brew install justincampbell/tap/revise
# or
go install github.com/justincampbell/revise@latest
```

Best in a tmux session — the skill opens revise in a popup. Outside tmux it falls back to asking the user to run revise themselves.

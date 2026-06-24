---
name: revise
description: Review local git changes (or a specific file) in the revise TUI, then act on the comments left behind. Use when the user wants to review the current diff, review a file/plan, or says "let me revise this", "open this in revise", or "/revise".
disable-model-invocation: true
---

# Revise

Open the [revise](https://github.com/justincampbell/revise) TUI so the user can review changes and leave inline comments, then read those comments back and work through them.

Requires the `revise` binary on PATH. If `command -v revise` fails, tell the user to install it (`brew install justincampbell/tap/revise` or `go install github.com/justincampbell/revise@latest`) and stop.

## Step 1: Decide what to review

- **No argument** → review the current working/branch diff. Run from inside a git repo (revise picks branch vs. working-tree mode automatically). If the cwd isn't a git repo, say so and stop.
- **A file path argument** (e.g. `/revise plan.md`) → review that single file in read-only file mode. Works anywhere, no git repo required.

## Step 2: Launch revise and wait for it to close

Pick a unique output file so comments can be read back:

```bash
outfile="$(mktemp -t revise-comments.XXXXXX)"
```

Build the command — **the `--output` flag must come before the positional file argument** (revise stops parsing flags at the first positional):

- Diff review: `revise --output "$outfile"`
- File review: `revise --output "$outfile" <file>`

**In tmux** (`$TMUX` is set), launch in a near-fullscreen popup that closes automatically when the user quits revise:

```bash
tmux display-popup -E -w 90% -h 90% -d "$PWD" "revise --output \"$outfile\""
```

`display-popup -E` **blocks until the popup closes**, so this call returns exactly when the user quits revise. Run it as a **background shell command** so the session isn't held by a tool timeout — it completes the moment the user quits, and you'll be notified.

**Not in tmux**, don't run revise in the agent's own terminal (it would fight for the screen). Instead tell the user to run the command themselves in another terminal/pane:

```
revise --output <outfile>          # or: revise --output <outfile> <file>
```

and let you know when they've quit. Then continue.

## Step 3: Read and act on the comments

When revise exits, read `$outfile`.

- **Empty or missing** → the user left no comments. Tell them there was nothing to act on and stop (don't invent work).
- Otherwise it's a Markdown document headed `# Code Review Comments`. Parse it:
  - `## <path>` starts a section for one file.
  - `<n>: ` `` `<source line>` `` followed by `> <comment>` is a comment anchored to line `<n>`. (`<n> (removed):` marks a deleted line.)
  - A bare `> <comment>` under a `##` header (no line number) is a **file-level** comment.
  - `> [flagged]` is a line the user flagged with no text — treat it as "look at this line," and if the intent isn't obvious, ask.

Then work through the feedback exactly as if the user had pasted it:

1. Group related comments into tasks — comments about the same topic, file, or change become one task, not several.
2. Track them (TaskCreate, or a TODO list) — mark each in-progress before starting, completed when done.
3. For each: read the referenced code, then make the requested change **or** explain why it's unnecessary.
4. When done, give a brief summary of what was addressed and anything you pushed back on.

Clean up the temp file when finished (`rm -f "$outfile"`).

Do not commit or push unless the user asks.

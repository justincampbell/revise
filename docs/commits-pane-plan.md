# Plan: Commits pane ("branch w/ only these commits" mode)

Status: **partially landed in #1** — the depth knob *and* a **read-only commit
list** (Commits section at the top of the file-list pane: in-scope `●` /
excluded `·`) shipped with the `<` / `>` work, because the knob wasn't usable
without seeing which commits were in scope. What remains here is the
**interactive** layer: toggling the pane, navigating it as the depth boundary,
and single-commit **isolate**.

Already done in #1 (reusable substrate): `git.BranchCommits()` / `git.CommitInfo`,
`branchCommitList` state, `setBranchCommits`/`syncBranchCommits`, the
`fileListModel` Commits section (`renderCommitsLines`/`renderCommitRow`,
`commitsSectionHeight`/`effectiveDepth`/`commitRowsShown`), and the
`branchCommitsMsg` startup priming.

## Goal

Let the reviewer scope a feature branch to **the last N commits** or **a single
commit**, by toggling the left pane from the file list to a list of the branch's
commits. The commit list is a visual front-end to the same `branchDepth` state
the `<` / `>` knob already drives.

## Decisions (locked)

- **Layout — toggle the left pane.** A key swaps the left pane between **Files**
  and **Commits**. No third pane, no overlay.
- **Selection — single commit + last-N.**
  - **Last-N (cumulative):** the cursor *is* the depth boundary. Cursor on row
    `k` (0-indexed from HEAD) ⇒ scope = last `k+1` commits + working tree.
  - **Single commit (isolate):** `Enter` scopes to just that one commit's diff.
- **Preview model — cursor = boundary, debounced live.** Moving the cursor sets
  `branchDepth` and schedules a reload **debounced (~120ms)** so a held `j`
  coalesces into one `git diff`. Reuse the existing `pollGen`-style generation
  counter so a newer move supersedes an in-flight reload. This keeps it live
  without thrashing shared worktrees (the 10–20-instances I/O concern).
- **`<` / `>` and the cursor share one source of truth** (`branchDepth`). The
  knob just moves the boundary; the list visualizes it.
- **Isolate is by SHA, not index.** Store the isolated commit's SHA; render via
  `RawDiffBetween(sha^, sha)`. A new commit landing mid-session (poll/rebase)
  must not silently re-point the selection. If the SHA disappears (squash/
  rebase), isolate clears gracefully on the next poll.
- **Isolate shows the pure committed diff — no working-tree overlay**, even for
  HEAD. A commit is immutable; isolate answers "what did *this commit* change."
  Working-tree review is what depth-0 / Staged / Unstaged already cover.
- **The scope drives the file list in both panes.** Toggle back to Files and you
  see the isolated commit's (or last-N's) files.
- **Commits pane only applies in Branch mode.** The toggle auto-switches to
  `ModeBranch` if needed, and is inert when 0 commits are ahead of the merge-base.
- **Row content:** `abc123  Fix parser  (2h ago)`. No `+/-` stats initially
  (they cost an extra `--numstat`/`--shortstat` call per row).
- **Status bar / slider:** isolate is a sub-state of Branch — the slider still
  says "Branch"; the status bar shows the detail (`last 2 of 5 commits` or
  `commit abc123 "Fix parser"`).

## Still open (confirm before building)

1. **Toggle key** — proposed `L` (mnemonic: *log*; `l`/`L` aren't taken for this).
   Alternative: backtick `` ` ``.
2. **Debounce window** — ~120ms proposed. Snappier/longer given multi-instance I/O?

## Interaction summary

| Key (in Commits pane) | Action |
|---|---|
| `j` / `k`, `↑` / `↓` | Move cursor = move the last-N boundary (debounced live reload) |
| `<` / `>` | Same as cursor down/up — adjust last-N (shared `branchDepth`) |
| `Enter` | Toggle **isolate** the commit under the cursor (pure single-commit diff) |
| `Esc` | Un-isolate (back to cumulative last-N); or close pane → Files |
| `L` (toggle key) | Swap left pane Files ↔ Commits |

## New plumbing

### `internal/git/`
- ~~`BranchCommits()`~~ **DONE in #1** — returns `[]CommitInfo{SHA, ShortSHA,
  Subject}`, newest first. (Relative age `RelAge` not yet added; add `%cr` to the
  `git log` format if the rows want it.)
- `CommitDiff(sha string) (*Diff, error)` — **remaining.** `RawDiffBetween(sha+"^", sha)`,
  parsed and `tagHunks(SourceBranch)`, **not** composed with the working tree.
- **Root-commit edge case:** if `sha^` has no parent (root commit == merge-base
  boundary at the repo root), diff against the empty tree
  (`git diff <empty-tree-hash> sha`, or `git show --format= sha`) so it doesn't
  error. Use `git hash-object -t tree /dev/null` / the well-known empty tree
  SHA `4b825dc642cb6eb9a060e54bf8d69288fbee4904`.

### `internal/ui/`
- The read-only Commits section already lives **inside** `fileListModel` (#1),
  not a separate model. The interactive version extends that: give the commit
  section its own cursor/focus when the pane is "active", reusing
  `renderCommitsLines`/`renderCommitRow`. (The original plan called for a
  separate `commitListModel`; folding it into the existing section is simpler.)
- `Model` state to add: `commitListActive bool`, `isolatedSHA string`.
  (`branchDepth` / `branchCommitList` already exist from #1.)
- Toggle key handler: auto-switch to `ModeBranch`, show/focus the commit
  section. Inert when `len(branchCommitList) == 0`.
- Cursor movement in the commit pane → set `branchDepth = cursor+1` (or 0 when
  at the full extent) → debounced reload via the generation counter.
- `Enter` → toggle `isolatedSHA`; when set, `loadDiff` routes to `CommitDiff`
  instead of `BranchDiffDepth`.
- `loadDiffWithOptions`: when `isolatedSHA != ""`, call `git.CommitDiff(sha)`;
  else the existing `BranchDiffDepth` path.
- Poll handler: refresh `commits`; if `isolatedSHA` no longer present, clear it.
- Status bar: extend `branchDepthHint()` (or a sibling) to show the isolate
  state.
- Slider/layout: render the commit pane in place of the file list when active.

### Keybinding (single source of truth)
- Add the toggle (and `Enter` isolate, if not already implied) to
  `internal/ui/keys.go` `allBindings` — feeds the `?` overlay and `--help`.
- All new bindings are `GitOnly: true` (disabled in file-review mode).

## Tests (TDD)

### `internal/git`
- `BranchCommits` returns the right count/order/subjects on a multi-commit
  feature branch; empty on the default branch / 0 ahead.
- `CommitDiff(sha)` returns exactly that commit's changes (not the cumulative
  range), and **excludes** uncommitted working-tree files.
- Root-commit isolate diffs against the empty tree without error.

### `internal/ui`
- Toggle activates the commit pane only in Branch mode (auto-switch from another
  mode) and is inert with 0 commits ahead.
- Cursor movement maps to `branchDepth` correctly (row k ⇒ depth k+1; full
  extent ⇒ 0).
- Reload is debounced/superseded (generation counter drops a stale in-flight
  reload) — assert via the existing tick/gen pattern.
- `Enter` sets `isolatedSHA` and routes `loadDiff` to `CommitDiff`; `Enter`
  again / `Esc` clears it.
- Isolate survives a new commit (pinned by SHA, not index); clears when the SHA
  disappears after a simulated squash/rebase poll.

## Docs to update on build

- `CLAUDE.md` — architecture map (new files), Key Decisions (Commits pane).
- `README.md` — Diff Modes / Key Bindings.
- `CHANGELOG.md` — Unreleased entry.
- `keys.go` help overlay grouping.

## Relationship to #1 (depth knob)

#1 already added: `BranchDiffDepth`, `CommitsAhead`, `BranchCommitCount`,
`branchFromRef` (git) and `branchDepth`/`branchCommits` state, `adjustBranchDepth`,
`setBranchCommits`, the `<` / `>` keys, the `branchCommitsMsg` startup priming,
and the `branchDepthHint` status segment (ui). The Commits pane reuses all of it
and adds the visual list + single-commit isolate on top.

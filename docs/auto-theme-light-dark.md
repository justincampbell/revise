# Auto theme: light/dark detection — investigation journal

Status as of **2026-06-10**: **partially working.** The rendering fix is solid;
**live** re-detection does **not** work in the reporter's setup
(`macOS → iTerm2 → tmux → revise`). Parked for now — this doc is the record to
resume from.

Related: issue #198, PR #199 (the rendering fix), and the
`bubbletea-v2-theme-detection` branch (the v2 upgrade + live-detection attempt).

## Goal

When the `auto` / `auto-daltonized` theme is active, `revise` should match the
terminal's light/dark background — both **at startup** and **live**, when the OS
appearance is toggled while `revise` is running.

## Current status

| Behavior | State |
| --- | --- |
| Startup detection (launch in light → light palette) | ✅ Works (incl. through tmux) |
| Light palette renders correctly (no dark code bg) | ✅ Fixed (#198 / PR #199) |
| Live switch, **no tmux** (terminal supports DEC mode 2031) | ✅ Should work (mode 2031 events) — not verified on a bare terminal |
| Live switch, **through tmux** (reporter's setup) | ❌ Not working |

## What was built

Three commits on `bubbletea-v2-theme-detection` (stacked on PR #199):

1. **`Adapt the auto theme to light terminals` (4b27b1a, = PR #199).** The real
   bug for #198: `autoPalette`/`autoDaltonizedPalette` ignored `isDark` and
   always returned dark diff-line backgrounds (`#1a2e1f` / `#2e1a1c`). On a light
   terminal the dark github-light syntax foreground was baked over a dark bg →
   unreadable ("code bg is always dark"). Fixed by mirroring the curated
   `github-light` palettes when `isDark` is false. **This works and is
   independently shippable.**

2. **`Upgrade to Bubble Tea v2` (5321bbd).** v1.3.10 has no way to learn about an
   OS appearance change at runtime. v2 surfaces DEC mode 2031 color-scheme
   reports as `uv.Light/DarkColorSchemeEvent`. Migration notes:
   - `tea.KeyMsg` → `tea.KeyPressMsg` (production already matched on `String()`).
   - `tea.MouseMsg` split into `MouseWheelMsg` / `MouseClickMsg` /
     `MouseReleaseMsg`; coords/button via `msg.Mouse()`.
   - `View() string` → `View() tea.View`; alt-screen / mouse / focus are set
     declaratively on the `tea.View` (`AltScreen`, `MouseMode`, `ReportFocus`),
     not as program options.
   - `Init` enables mode 2031 via `tea.Raw(ansi.SetModeLightDark)` for auto
     themes; `Update` routes the color-scheme events through `applyColorScheme`
     (re-`SetTheme` + rebuild diff lines; auto themes only; only when the scheme
     flipped).

3. **`Re-detect light/dark on focus` (d842bf3).** Because mode 2031 doesn't make
   it through tmux (see below), auto themes also re-query the background on
   `FocusMsg` via `tea.RequestBackgroundColor`. Added `theme: <name>/<light|dark>`
   to the `--debug` strip for diagnosis.

## Root-cause analysis

The reporter's stack is `macOS → iTerm2 → tmux → revise`. The signal has to
survive every layer.

- **DEC mode 2031 (instant OS push):** `revise` enables it, but **tmux does not
  forward** the unsolicited light/dark reports to the pane. So the
  `uv.*ColorSchemeEvent` path never fires under tmux. (It should work on a bare
  terminal that supports 2031 — Ghostty, kitty, WezTerm — but that's unverified.
  iTerm2's own 2031 support is also unconfirmed.)

- **Focus + OSC 11 re-query (the fallback we shipped):** Two things *do* cross
  tmux: focus events (the reporter has `set -g focus-events on`) and OSC 11
  background-color queries (startup detection proves the round-trip works at
  least once). So on refocus we re-query the background. **This still didn't work
  for the reporter.**

### Leading hypothesis for why the focus fallback fails

**tmux likely caches the OSC 11 background color** rather than re-querying iTerm
on each request. So the focus-time `RequestBackgroundColor` comes back with the
*startup* value, not the post-toggle one — `applyColorScheme` sees no change and
does nothing. (Unconfirmed — needs the `--debug` check below.)

Other possibilities to rule out:
- iTerm2 profile not actually changing its background with the OS appearance
  (unlikely — startup detection distinguishes light/dark for the reporter, so
  iTerm *does* report different bg per appearance at launch).
- The refocus event not firing because the OS-appearance toggle didn't actually
  blur/refocus the tmux pane (depends on how the user toggles).

## How to diagnose (do this first when resuming)

```sh
revise --debug
```

The dev strip ends with `theme: auto/dark` (or `auto/light`). Toggle the OS
appearance, click away from iTerm and back, and watch that field:

- **Flips** → re-detection works; the earlier failure was likely a stale build
  or a non-refocusing toggle.
- **Stays put on refocus** → confirms the **tmux OSC 11 caching** hypothesis.
  Query-based detection cannot work through this tmux; pursue the options below.

Also worth capturing: `tmux -V` (reporter is on **3.5a**) and
`tmux show-options -g | grep -iE 'focus|passthrough|terminal-features'`
(reporter has `focus-events on`, `terminal-features ',*:RGB'`, no
`allow-passthrough`).

## Next things to try (when resuming)

1. **Confirm the hypothesis** with `--debug` as above. Everything else depends on
   whether OSC 11 returns a fresh value through tmux.
2. **Check whether tmux 3.5 can forward mode 2031.** If a tmux option /
   `terminal-features` / `allow-passthrough` makes tmux relay the 2031 reports
   (or re-query OSC 11 live), document the required `~/.tmux.conf` line — that may
   be the whole fix, no code change.
3. **Verify on a bare terminal** (Ghostty / kitty / WezTerm, no tmux). If the
   mode-2031 path works there, the feature is correct and the limitation is
   tmux-specific — worth stating plainly in the README.
4. **Reconsider whether the v2 upgrade is worth keeping** if it can't deliver the
   live behavior in the reporter's primary setup. The v2 migration is clean and
   has upsides, but it isn't required for the (working) rendering fix in PR #199.
   Option: merge #199 on its own; keep or drop the v2 branch based on (1)–(3).
5. **Manual re-detect key** as a last resort — but it shares the OSC 11 path, so
   it only helps if (1) shows fresh values; it won't beat tmux caching.

## Key code locations

- `internal/ui/theme.go` — `autoPalette` / `autoDaltonizedPalette` adapt to
  `isDark`; `paletteFor`; `chromaStyleEntries`.
- `internal/ui/syntax.go` — `highlightLine` chroma-style selection per theme.
- `internal/ui/model.go`:
  - `Init` — enables mode 2031 + initial `RequestBackgroundColor` for auto themes.
  - `Update` — `uv.Light/DarkColorSchemeEvent` and `tea.BackgroundColorMsg` →
    `applyColorScheme`; `FocusMsg` → `RequestBackgroundColor`.
  - `applyColorScheme` — the single re-theme entry point.
  - `View` — sets `AltScreen` / `MouseMode` / `ReportFocus` on the `tea.View`.
  - `renderDebugStrip` — the `theme:` field.
- `main.go` — pre-program `lipgloss.HasDarkBackground` for the initial value;
  `tea.NewProgram` options.
- `internal/ui/model_test.go` — `TestColorSchemeEvent_AutoThemeTracksOS`,
  `TestFileReviewMode_Init`, `pinExplicitTheme` helper.

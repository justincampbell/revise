package ui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	commentstore "github.com/justincampbell/revise/internal/comments"
	"github.com/justincampbell/revise/internal/editor"
	"github.com/justincampbell/revise/internal/fswatch"
	"github.com/justincampbell/revise/internal/git"
	"github.com/justincampbell/revise/internal/refresh"
	"github.com/justincampbell/revise/internal/update"
)

type clearStatusMsg struct{}

// refreshTickMsg triggers a fingerprint check. The gen field is a
// generation counter — when requestRefresh schedules a new poll, it
// bumps gen so any earlier in-flight tea.Tick is invalidated. This
// removes the dead zone where a long-pending tick would block a more
// recent focus/fsEvent from firing right away.
type refreshTickMsg struct {
	gen uint64
}

// refreshResultMsg carries the result of a fingerprint check, plus how
// long it took. The duration feeds the adaptive cadence in
// refresh.Policy: slow polls schedule the next tick further out so 10-20
// concurrent revise instances on shared-repo worktrees don't pile up I/O.
type refreshResultMsg struct {
	fingerprint string
	err         error
	duration    time.Duration
}

// fsEventMsg is delivered when fswatch.Watcher reports a change. The
// handler must re-issue the listen command in exactly one place to keep
// the channel pump alive.
type fsEventMsg struct{}

// hunkApplyMsg is sent when an async hunk stage/unstage/discard completes.
type hunkApplyMsg struct {
	err error
}

// updateCheckMsg carries the result of a background update check.
type updateCheckMsg struct {
	info *update.UpdateInfo
	err  error
}

// untrackedCacheTipMsg fires if the repo has core.untrackedCache disabled —
// we surface a one-time tip. We avoid the FS support test on startup because
// `git update-index --test-untracked-cache` leaves mtime-test-XXXXXX/ dirs in
// CWD if the process is killed mid-test; the real test runs in `setup-cache`.
type untrackedCacheTipMsg struct{}

// updateAppliedMsg is sent when an update download+install completes.
type updateAppliedMsg struct {
	newVersion string
	err        error
}

// editorFinishedMsg is sent when the $EDITOR process exits after `E`.
// The handler reloads the diff so any edits the user made are visible
// immediately on return (file review mode skips the reload since there's
// no diff to refresh; the user can `r` if needed).
type editorFinishedMsg struct {
	err error
}

// diffLoadedMsg is sent when an async diff load completes.
type diffLoadedMsg struct {
	diff            *git.Diff
	err             error
	onDefaultBranch bool
	noCommits       bool          // true if the repo has no commits at load time
	fromPoll        bool          // true when triggered by auto-refresh polling
	duration        time.Duration // time the diff load took (used by --debug overlay)
}

// debugTickMsg fires once per second in --debug mode so the debug strip
// can re-render with up-to-date "Xs ago" / "next in Ys" values without
// waiting for an unrelated message to redraw the screen.
type debugTickMsg struct{}

// DiffMode represents which diff view is active.
// Modes are cumulative: Unstaged is always included,
// broader modes add Staged, then Branch.
type DiffMode int

const (
	ModeBranch     DiffMode = iota // committed + staged + unstaged + untracked (broadest, feature branch only)
	ModeStaged                     // staged + unstaged + untracked
	ModeStagedOnly                 // staged only (no unstaged or untracked)
	ModeUnstaged                   // unstaged + untracked only (narrowest)
)

type focusPanel int

const (
	focusFileList focusPanel = iota
	focusDiffView
)

const fileListWidth = 30

// hScrollStep is the horizontal scroll distance per key/wheel event.
const hScrollStep = 8

type Model struct {
	diff       *git.Diff
	fileList   fileListModel
	diffView   diffViewModel
	focus      focusPanel
	showHelp   bool
	helpScroll int
	fullscreen bool
	width      int
	height     int
	ready      bool

	mode              DiffMode
	modeExplicitlySet bool // true once the user has manually picked a mode (#148)
	onDefaultBranch   bool
	noCommits         bool // true when the repo has no commits yet (#189)
	contextLines      int
	hideWhitespace    bool

	comments           comments
	marks              marks
	storePath          string // path to JSON file for comment persistence
	commentInputActive bool
	commentTarget      commentKey
	statusMsg          string

	confirmDiscard    bool    // waiting for confirmation to discard
	confirmMsg        string  // message shown in the discard confirmation modal
	pendingDiscardCmd tea.Cmd // the discard command to run on confirmation

	confirmClear bool // waiting for confirmation to clear all comments

	// Auto-refresh state. The flow is:
	//   FocusMsg / fsEventMsg / scheduled tick → requestRefresh()
	//     → refreshTickMsg (if not stale, not polling, focused)
	//     → fingerprint check → refreshResultMsg
	//     → if changed: loadDiffFromPoll()
	//     → schedule next tick via refreshPolicy.NextDelay(lastPollDuration)
	// pollGen invalidates earlier-scheduled ticks (see refreshTickMsg).
	refreshPolicy        refresh.Policy
	lastFingerprint      string
	lastPollStart        time.Time
	lastPollDuration     time.Duration
	lastDiffLoadStart    time.Time
	lastDiffLoadDuration time.Duration
	nextTickAt           time.Time // when the next scheduled refreshTickMsg is due (for --debug overlay)
	polling              bool
	focused              bool
	pollGen              uint64
	fsWatcher            *fswatch.Watcher // nil if disabled or init failed
	debug                bool             // true in --debug mode; renders refresh debug strip

	currentVersion string             // version string from build
	updateInfo     *update.UpdateInfo // populated after update check
	updating       bool               // true while downloading update

	fileReviewMode bool   // true when reviewing a file (not a git diff)
	fileReviewPath string // filename shown in status bar
}

// NewFileReview creates a Model for reviewing a file (not a git diff).
// It starts in fullscreen with focus on the diff view.
func NewFileReview(diff *git.Diff, filePath string) Model {
	fl := newFileListModel(diff.Files)
	dv := newDiffViewModel()

	c := make(comments)
	dv.comments = c
	dv.fileReviewMode = true
	fl.comments = c

	if len(diff.Files) > 0 {
		dv.setFile(&diff.Files[0])
	}

	return Model{
		diff:           diff,
		fileList:       fl,
		diffView:       dv,
		focus:          focusDiffView,
		fullscreen:     true,
		comments:       c,
		mode:           ModeStaged,
		fileReviewMode: true,
		fileReviewPath: filePath,
	}
}

func New(diff *git.Diff, onDefaultBranch bool) Model {
	return NewWithStorePath(diff, onDefaultBranch, "", "")
}

// NewWithStorePath creates a Model with comment persistence at the given path.
// If storePath is non-empty, comments are loaded from it on startup and saved
// automatically on every add/edit/delete.
func NewWithStorePath(diff *git.Diff, onDefaultBranch bool, storePath string, version string) Model {
	fl := newFileListModel(diff.Files)
	dv := newDiffViewModel()

	c := make(comments)
	mk := make(marks)

	// Load persisted comments and marks if a store path is configured.
	if storePath != "" {
		if loaded, err := commentstore.Load(storePath); err == nil {
			c, mk = fromStringMap(loaded)
		}
	}

	dv.comments = c
	dv.marks = mk
	fl.comments = c
	fl.marks = mk

	if len(diff.Files) > 0 {
		dv.setFile(&diff.Files[0])
	}

	mode := ModeBranch
	if onDefaultBranch {
		mode = ModeStaged
	}

	// Capture initial fingerprint so the first poll tick doesn't
	// trigger an unnecessary reload.
	initialFP, _ := git.StatusFingerprint()

	return Model{
		diff:            diff,
		fileList:        fl,
		diffView:        dv,
		focus:           focusFileList,
		comments:        c,
		marks:           mk,
		storePath:       storePath,
		mode:            mode,
		onDefaultBranch: onDefaultBranch,
		noCommits:       !git.HasCommits(),
		contextLines:    git.DefaultContextLines,
		refreshPolicy:   refresh.Default,
		lastFingerprint: initialFP,
		focused:         true,
		currentVersion:  version,
	}
}

// WithFSWatcher attaches an fswatch.Watcher so file changes wake the
// refresh loop directly, instead of waiting for the next scheduled tick.
// Pass nil (or skip the call) for timer-only auto-refresh — that's the
// fallback when fswatch.New fails or the user passed --no-watch.
func (m Model) WithFSWatcher(w *fswatch.Watcher) Model {
	m.fsWatcher = w
	return m
}

// WithDebug enables the refresh-debug strip above the status bar. It
// shows live refresh timings: time since the last fingerprint check and
// diff reload, durations of those operations, time until the next
// scheduled tick, and whether fsnotify is wired up. Useful for tuning
// refresh.Policy or debugging "why isn't this updating".
//
// Independent of --dev: --dev auto-restarts on binary replacement,
// --debug surfaces refresh internals. They compose freely.
func (m Model) WithDebug(debug bool) Model {
	m.debug = debug
	return m
}

func (m Model) Init() tea.Cmd {
	if m.fileReviewMode {
		return nil
	}
	// Schedule the first auto-refresh tick. pollGen=0 so a fresh tick
	// (also gen=0) is accepted; subsequent requestRefresh calls bump
	// the counter and supersede this one.
	cmds := []tea.Cmd{
		tea.Tick(m.refreshPolicy.Min, func(time.Time) tea.Msg {
			return refreshTickMsg{gen: 0}
		}),
	}
	if cmd := listenFSEvents(m.fsWatcher); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if m.debug {
		cmds = append(cmds, tea.Tick(time.Second, func(time.Time) tea.Msg { return debugTickMsg{} }))
	}
	if m.currentVersion != "" && update.IsSemver(m.currentVersion) {
		ver := m.currentVersion
		cmds = append(cmds, func() tea.Msg {
			info, err := update.CheckForUpdate(ver, false)
			return updateCheckMsg{info: info, err: err}
		})
	}
	cmds = append(cmds, func() tea.Msg {
		if git.UntrackedCacheEnabled() {
			return nil
		}
		return untrackedCacheTipMsg{}
	})
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.commentInputActive {
			return m.updateCommentInput(msg)
		}

		if m.confirmDiscard {
			return m.updateConfirmDiscard(msg)
		}

		if m.confirmClear {
			return m.updateConfirmClear(msg)
		}

		if m.showHelp {
			switch msg.String() {
			case "j", "down":
				if m.helpScroll < helpMaxScroll(m.height) {
					m.helpScroll++
				}
				return m, nil
			case "k", "up":
				if m.helpScroll > 0 {
					m.helpScroll--
				}
				return m, nil
			default:
				m.showHelp = false
				m.helpScroll = 0
				return m, nil
			}
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "?":
			m.showHelp = !m.showHelp
			return m, nil

		// Soft wrap toggle — works in both git diff and file review mode.
		// No diff reload needed; it's purely a rendering change.
		case "W", "alt+z":
			m.diffView.toggleWrap()
			return m, nil

		case "right", "l":
			if m.fileReviewMode {
				m.diffView.scrollRight(hScrollStep)
				return m, nil
			}
			if m.focus == focusDiffView {
				m.diffView.scrollRight(hScrollStep)
			} else {
				m.focus = focusDiffView
			}
			return m, nil

		case "left", "h":
			if m.fileReviewMode {
				m.diffView.scrollLeft(hScrollStep)
				return m, nil
			}
			if m.focus == focusDiffView && m.diffView.hOffset > 0 {
				m.diffView.scrollLeft(hScrollStep)
				return m, nil
			}
			if m.fullscreen {
				m.fullscreen = false
				m.updateLayout()
			}
			m.focus = focusFileList
			return m, nil

		case "f", "esc":
			if m.fileReviewMode {
				return m, nil
			}
			switch msg.String() {
			case "f":
				m.fullscreen = !m.fullscreen
				if m.fullscreen {
					m.focus = focusDiffView
				}
				m.updateLayout()
			case "esc":
				if m.fullscreen {
					m.fullscreen = false
					m.updateLayout()
				}
				m.focus = focusFileList
			}
			return m, nil

		// Git-only keys — no-op in file review mode
		case "tab", "shift+tab", "w", "+", "=", "-", "_":
			if m.fileReviewMode {
				return m, nil
			}
			switch msg.String() {
			case "tab":
				m.cycleMode(+1)
				return m, m.loadDiff()
			case "shift+tab":
				m.cycleMode(-1)
				return m, m.loadDiff()
			case "w":
				m.hideWhitespace = !m.hideWhitespace
				return m, m.loadDiff()
			case "+", "=":
				m.contextLines++
				return m, m.loadDiff()
			case "-", "_":
				if m.contextLines > 0 {
					m.contextLines--
					return m, m.loadDiff()
				}
				return m, nil
			}

		// File list navigation (always works)
		case "n":
			m.nextFile()
			return m, nil
		case "N":
			m.prevFile()
			return m, nil

		// Hard refresh — reload diff regardless of fingerprint, so the user
		// has an escape hatch when auto-refresh hasn't picked up a change.
		case "r":
			if m.fileReviewMode {
				return m, nil
			}
			m.statusMsg = "Refreshing…"
			return m, tea.Batch(
				m.loadDiff(),
				tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} }),
			)

		// Open current file at the cursor line in $EDITOR (suspends the TUI).
		case "E":
			if cmd := m.openInEditor(); cmd != nil {
				return m, cmd
			}
			return m, nil

		// Report issue opens GitHub new issue page
		case "!":
			const issueURL = "https://github.com/justincampbell/revise/issues/new/choose"
			m.statusMsg = "Opening " + issueURL
			return m, tea.Batch(
				func() tea.Msg {
					_ = exec.Command("open", issueURL).Start()
					return nil
				},
				tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} }),
			)

		// Clear all comments
		case "C":
			if len(m.comments) > 0 || len(m.marks) > 0 {
				n := len(m.comments) + len(m.marks)
				noun := "items"
				if n == 1 {
					noun = "item"
				}
				m.confirmClear = true
				m.confirmMsg = fmt.Sprintf("Clear all comments and marks? (%d %s)", n, noun)
			}
			return m, nil

		// Apply update
		case "ctrl+u":
			if m.updating {
				m.statusMsg = "Update in progress..."
				return m, nil
			}
			if m.updateInfo == nil || !m.updateInfo.IsNewer {
				m.statusMsg = "No update available"
				return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
			}
			m.updating = true
			m.statusMsg = "Downloading update..."
			return m, func() tea.Msg {
				err := update.ApplyUpdate()
				ver := ""
				if m.updateInfo != nil {
					ver = m.updateInfo.LatestVersion
				}
				return updateAppliedMsg{newVersion: ver, err: err}
			}

		// Export works from any panel
		case "e":
			msg := m.exportComments()
			if msg != "" {
				m.statusMsg = msg
				return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
			}
			return m, nil
		}

		if m.focus == focusFileList {
			return m.updateFileList(msg)
		}
		return m.updateDiffView(msg)

	case tea.MouseMsg:
		if m.showHelp {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				if m.helpScroll > 0 {
					m.helpScroll--
				}
			case tea.MouseButtonWheelDown:
				if m.helpScroll < helpMaxScroll(m.height) {
					m.helpScroll++
				}
			}
			return m, nil
		}

		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if m.mouseFocusDiff(msg) {
				m.diffView.scrollUp(3)
			} else {
				m.fileList.moveUp()
				m.syncSelectedFile()
			}
		case tea.MouseButtonWheelDown:
			if m.mouseFocusDiff(msg) {
				m.diffView.scrollDown(3)
			} else {
				m.fileList.moveDown()
				m.syncSelectedFile()
			}
		case tea.MouseButtonWheelLeft:
			if m.mouseFocusDiff(msg) {
				m.diffView.scrollLeft(hScrollStep)
			}
		case tea.MouseButtonWheelRight:
			if m.mouseFocusDiff(msg) {
				m.diffView.scrollRight(hScrollStep)
			}
		case tea.MouseButtonLeft:
			if msg.Action != tea.MouseActionPress {
				return m, nil
			}
			// Check for status bar slider click
			if msg.Y == m.height-1 {
				if mode := m.sliderModeAt(msg.X); mode >= 0 && mode != m.mode && m.modeAvailable(mode) {
					m.mode = mode
					m.modeExplicitlySet = true
					return m, m.loadDiff()
				}
				return m, nil
			}
			if !m.mouseFocusDiff(msg) {
				// border (1) = 1 line before file entries
				if !m.commentInputActive {
					idx := msg.Y - 1 + m.fileList.offset
					if idx >= 0 && idx < len(m.fileList.files) {
						m.fileList.cursor = idx
						m.syncSelectedFile()
					}
				}
			} else {
				m.focus = focusDiffView
				// Panel top border is 1 row; map click Y to lines[] index.
				clickY := msg.Y - 1
				if clickY >= 0 {
					absIdx := m.diffView.clickToAbsIdx(clickY)
					// Close any open input box before navigating.
					if m.commentInputActive {
						m.commentInputActive = false
						m.diffView.commentInputActive = false
						m.diffView.fileCommentInput = false
						m.diffView.textInput.Blur()
					}
					if absIdx >= 0 && absIdx < len(m.diffView.lines) {
						m.diffView.cursor = absIdx
						// Step back from non-navigable lines to the nearest code line.
						for m.diffView.cursor > 0 && !m.diffView.isNavigable(m.diffView.cursor) {
							m.diffView.cursor--
						}
						if m.diffView.cursorRef() != nil {
							// Click in gutter area toggles mark; elsewhere opens comment.
							diffPanelX := m.fileList.width + 2
							if m.fullscreen {
								diffPanelX = 0
							}
							// border (1) + cursor prefix (1) + gutter (6) = 8
							if msg.X < diffPanelX+8 {
								m.toggleMarkAtCursor()
							} else {
								m.startCommentInput()
							}
						}
					}
				}
			}
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.updateLayout()
		return m, nil

	case diffLoadedMsg:
		m.lastDiffLoadDuration = msg.duration
		if msg.err != nil {
			m.statusMsg = "Error: " + msg.err.Error()
			return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
		}
		m.diff = msg.diff
		m.noCommits = msg.noCommits

		// Update default-branch status so Branch mode becomes
		// available (or unavailable) dynamically.
		wasOnDefaultBranch := m.onDefaultBranch
		m.onDefaultBranch = msg.onDefaultBranch
		// When the branch diverges from default while revise is running
		// (e.g. user committed on a fresh branch), promote mode to
		// ModeBranch so the new commits are visible — but only if the
		// user hasn't explicitly picked a different mode (see #148).
		// The diff in this msg was loaded under the OLD mode, so kick off
		// a follow-up reload under the new mode; otherwise the user sees
		// Branch mode with stale (often empty) contents.
		var followup tea.Cmd
		if wasOnDefaultBranch && !m.onDefaultBranch && !m.modeExplicitlySet {
			m.mode = ModeBranch
			followup = m.loadDiffFromPoll()
		}
		// If the current mode is no longer valid, clamp it.
		modes := m.availableModes()
		modeValid := false
		for _, am := range modes {
			if am == m.mode {
				modeValid = true
				break
			}
		}
		if !modeValid {
			m.mode = modes[0]
		}

		selectedPath := ""
		prevCursor := m.fileList.cursor
		if f := m.fileList.selectedFile(); f != nil {
			selectedPath = f.Path
		}
		m.fileList = newFileListModel(m.diff.Files)
		m.fileList.comments = m.comments
		m.fileList.marks = m.marks
		m.updateLayout()
		// Re-select the same file if it still exists, otherwise
		// keep the cursor at the same index (clamped to the list).
		found := false
		if selectedPath != "" {
			for i, f := range m.diff.Files {
				if f.Path == selectedPath {
					m.fileList.cursor = i
					found = true
					break
				}
			}
		}
		if !found && len(m.diff.Files) > 0 {
			if prevCursor >= len(m.diff.Files) {
				m.fileList.cursor = len(m.diff.Files) - 1
			} else {
				m.fileList.cursor = prevCursor
			}
		}
		if msg.fromPoll {
			// Preserve scroll position on auto-refresh.
			savedCursor := m.diffView.cursor
			savedOffset := m.diffView.offset
			m.syncSelectedFile()
			m.diffView.cursor = savedCursor
			m.diffView.offset = savedOffset
		} else {
			m.syncSelectedFile()
			m.diffView.offset = 0
			m.diffView.goToTop()
		}
		return m, followup

	case hunkApplyMsg:
		if msg.err != nil {
			m.statusMsg = "Error: " + msg.err.Error()
			return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
		}
		return m, m.loadDiff()

	case editorFinishedMsg:
		// Editor exited cleanly OR with a non-zero exit code (e.g. `:cq`
		// in vim, which users do intentionally to signal "cancel"). Both
		// are normal — only flag the message if exec itself failed
		// (binary not found, permission denied, etc.).
		var exitErr *exec.ExitError
		if msg.err != nil && !errors.As(msg.err, &exitErr) {
			m.statusMsg = "Editor error: " + msg.err.Error()
			return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
		}
		if m.fileReviewMode {
			return m, nil
		}
		return m, m.loadDiff()

	case clearStatusMsg:
		m.statusMsg = ""
		return m, nil

	case debugTickMsg:
		// In --debug mode, drive a once-per-second redraw so the debug
		// strip's relative timestamps stay current. Outside --debug the
		// tick is never scheduled, so this is dead code at runtime.
		if !m.debug {
			return m, nil
		}
		return m, tea.Tick(time.Second, func(time.Time) tea.Msg { return debugTickMsg{} })

	case tea.FocusMsg:
		m.focused = true
		return m, m.requestRefresh()

	case tea.BlurMsg:
		m.focused = false
		return m, nil

	case fsEventMsg:
		// File changed under the watcher. Request a refresh (subject to
		// debounce) and re-issue the listen Cmd to keep the channel
		// pump alive. The re-issue lives here, in exactly one place.
		return m, tea.Batch(m.requestRefresh(), listenFSEvents(m.fsWatcher))

	case refreshTickMsg:
		// Drop ticks superseded by a later requestRefresh (gen counter).
		// Drop if a check is already in flight or we're unfocused.
		if msg.gen != m.pollGen || m.polling || !m.focused {
			return m, nil
		}
		m.polling = true
		m.lastPollStart = time.Now()
		start := m.lastPollStart
		return m, func() tea.Msg {
			fp, err := git.StatusFingerprint()
			return refreshResultMsg{
				fingerprint: fp,
				err:         err,
				duration:    time.Since(start),
			}
		}

	case untrackedCacheTipMsg:
		m.statusMsg = "Tip: enable core.untrackedCache for faster refresh — run 'revise setup-cache'"
		return m, tea.Tick(10*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })

	case updateCheckMsg:
		if msg.err == nil && msg.info != nil && msg.info.IsNewer {
			m.updateInfo = msg.info
			m.statusMsg = fmt.Sprintf("Update available: %s → %s (", msg.info.CurrentVersion, msg.info.LatestVersion) +
				helpKeyStyle.Render("Ctrl+U") + statusBarStyle.Render(" to update)")
			return m, tea.Tick(10*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
		}
		return m, nil

	case updateAppliedMsg:
		m.updating = false
		if msg.err != nil {
			m.statusMsg = "Update failed: " + msg.err.Error()
			return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
		}
		m.statusMsg = fmt.Sprintf("Updated to %s — restart revise to use the new version", msg.newVersion)
		return m, tea.Tick(10*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })

	case refreshResultMsg:
		m.polling = false
		m.lastPollDuration = msg.duration
		// Schedule next tick at policy.NextDelay(duration) so cadence
		// self-throttles under contention (10-20 instances). On blur
		// we don't reschedule; FocusMsg will request the next refresh.
		var nextTick tea.Cmd
		if m.focused {
			m.pollGen++
			gen := m.pollGen
			delay := m.refreshPolicy.NextDelay(msg.duration)
			m.nextTickAt = time.Now().Add(delay)
			nextTick = tea.Tick(delay, func(time.Time) tea.Msg {
				return refreshTickMsg{gen: gen}
			})
		} else {
			m.nextTickAt = time.Time{}
		}
		if msg.err != nil {
			return m, nextTick
		}
		if msg.fingerprint != m.lastFingerprint {
			m.lastFingerprint = msg.fingerprint
			if nextTick != nil {
				return m, tea.Batch(m.loadDiffFromPoll(), nextTick)
			}
			return m, m.loadDiffFromPoll()
		}
		return m, nextTick
	}

	return m, nil
}

func (m *Model) updateLayout() {
	panelH := m.height - 3 // leave 1 row for status bar
	if m.debug {
		panelH-- // debug strip sits directly above the status bar
	}

	if m.fullscreen {
		m.diffView.width = m.width - 2
		m.diffView.height = panelH
		return
	}

	listW := fileListWidth
	if m.width < 80 {
		listW = m.width / 3
	}

	m.fileList.width = listW
	m.fileList.height = panelH
	m.diffView.width = m.width - listW - 3 // file list right border (1) + gap (1) + diff left border (1)
	m.diffView.height = panelH
}

func (m Model) updateFileList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.fileList.moveDown()
		m.syncSelectedFile()
	case "k", "up":
		m.fileList.moveUp()
		m.syncSelectedFile()
	case "enter":
		m.syncSelectedFile()
		m.focus = focusDiffView
	// Git-only keys — no-op in file review mode
	case "s", "S", "u", "U", "D":
		if m.fileReviewMode {
			return m, nil
		}
		switch msg.String() {
		case "s", "S":
			if cmd := m.stageCurrentFile(); cmd != nil {
				return m, cmd
			}
		case "u", "U":
			if cmd := m.unstageCurrentFile(); cmd != nil {
				return m, cmd
			}
		case "D":
			if cmd := m.discardCurrentFile(); cmd != nil {
				return m, cmd
			}
		}
	case "c":
		m.startFileComment()
	case "d":
		m.deleteFileComment()
	}
	return m, nil
}

func (m Model) updateDiffView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.diffView.moveCursorDown(1)
	case "k", "up":
		m.diffView.moveCursorUp(1)
	case "g", "home":
		m.diffView.goToTop()
	case "G", "end":
		m.diffView.goToBottom()
	case "pgdown", " ":
		m.diffView.pageDown()
	case "pgup":
		m.diffView.pageUp()
	case "}", "]":
		m.diffView.nextHunk()
	case "{", "[":
		m.diffView.prevHunk()
	case "c", "enter":
		if m.diffView.cursorRef() != nil {
			m.startCommentInput()
		}
	case "m":
		m.toggleMarkAtCursor()
	case "d":
		m.deleteCommentAtCursor()
	// Git-only keys — no-op in file review mode
	case "s", "u", "D", "S", "U":
		if m.fileReviewMode {
			return m, nil
		}
		switch msg.String() {
		case "s":
			if cmd := m.stageCurrentHunk(); cmd != nil {
				return m, cmd
			}
		case "u":
			if cmd := m.unstageCurrentHunk(); cmd != nil {
				return m, cmd
			}
		case "D":
			if cmd := m.discardCurrentHunk(); cmd != nil {
				return m, cmd
			}
		case "S":
			if cmd := m.stageCurrentFile(); cmd != nil {
				return m, cmd
			}
		case "U":
			if cmd := m.unstageCurrentFile(); cmd != nil {
				return m, cmd
			}
		}
	}
	return m, nil
}

func (m Model) updateCommentInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		text := strings.TrimSpace(m.diffView.textInput.Value())
		if text != "" {
			m.comments[m.commentTarget] = text
		} else {
			delete(m.comments, m.commentTarget)
		}
		m.saveComments()
		m.commentInputActive = false
		m.diffView.commentInputActive = false
		m.diffView.fileCommentInput = false
		m.diffView.textInput.Blur()
		m.diffView.rebuildLinesPreservingCursor()
	case "esc":
		m.commentInputActive = false
		m.diffView.commentInputActive = false
		m.diffView.fileCommentInput = false
		m.diffView.textInput.Blur()
	default:
		var cmd tea.Cmd
		m.diffView.textInput, cmd = m.diffView.textInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) updateConfirmClear(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.confirmClear = false
	m.confirmMsg = ""
	switch msg.String() {
	case "y", "Y", "enter":
		for k := range m.comments {
			delete(m.comments, k)
		}
		for k := range m.marks {
			delete(m.marks, k)
		}
		m.saveComments()
		m.diffView.rebuildLinesPreservingCursor()
	}
	return m, nil
}

func (m Model) updateConfirmDiscard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.confirmDiscard = false
	m.confirmMsg = ""
	switch msg.String() {
	case "y", "Y", "enter":
		return m, m.pendingDiscardCmd
	default:
		m.pendingDiscardCmd = nil
		return m, nil
	}
}

// saveComments persists comments to the configured store path (if any).
func (m *Model) saveComments() {
	if m.storePath == "" {
		return
	}
	_ = commentstore.Save(m.storePath, toStringMap(m.comments, m.marks))
}

func (m *Model) startCommentInput() {
	ref := m.diffView.cursorRef()
	if ref == nil || m.diffView.file == nil {
		return
	}
	key := ref.commentKey(m.diffView.file.Path)
	m.commentTarget = key

	m.diffView.textInput.SetValue(m.comments[key])
	m.diffView.textInput.CursorEnd()
	m.diffView.textInput.Focus()

	// Scroll so there's room below the cursor for the input box.
	m.diffView.scrollForCommentInput()

	m.commentInputActive = true
	m.diffView.commentInputActive = true
}

func (m *Model) startFileComment() {
	f := m.fileList.selectedFile()
	if f == nil {
		return
	}
	key := commentKey{file: f.Path, lineNum: 0}
	m.commentTarget = key

	m.diffView.textInput.SetValue(m.comments[key])
	m.diffView.textInput.CursorEnd()
	m.diffView.textInput.Focus()

	m.commentInputActive = true
	m.diffView.commentInputActive = true
	m.diffView.fileCommentInput = true
}

func (m *Model) deleteFileComment() {
	f := m.fileList.selectedFile()
	if f == nil {
		return
	}
	key := commentKey{file: f.Path, lineNum: 0}
	if _, ok := m.comments[key]; ok {
		delete(m.comments, key)
		m.saveComments()
		m.diffView.rebuildLinesPreservingCursor()
	}
}

func (m *Model) toggleMarkAtCursor() {
	ref := m.diffView.cursorRef()
	if ref == nil || m.diffView.file == nil {
		return
	}
	key := ref.commentKey(m.diffView.file.Path)
	if m.marks[key] {
		delete(m.marks, key)
	} else {
		m.marks[key] = true
	}
	m.saveComments()
	m.diffView.rebuildLinesPreservingCursor()
}

func (m *Model) deleteCommentAtCursor() {
	ref := m.diffView.cursorRef()
	if ref == nil || m.diffView.file == nil {
		return
	}
	key := ref.commentKey(m.diffView.file.Path)
	delete(m.comments, key)
	delete(m.marks, key)
	m.saveComments()
	m.diffView.rebuildLinesPreservingCursor()
}

// stageCurrentHunk stages the hunk at the cursor. Only works on unstaged hunks.
func (m *Model) stageCurrentHunk() tea.Cmd {
	idx := m.diffView.currentHunkIndex()
	if idx < 0 || m.diffView.file == nil {
		return nil
	}
	hunk := m.diffView.file.Hunks[idx]
	if hunk.Source != git.SourceUnstaged {
		m.statusMsg = "Can only stage unstaged hunks"
		return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
	}
	path := m.diffView.file.Path
	status := m.diffView.file.Status
	return func() tea.Msg {
		err := git.StageHunk(path, status, hunk)
		return hunkApplyMsg{err: err}
	}
}

// unstageCurrentHunk unstages the hunk at the cursor. Only works on staged hunks.
func (m *Model) unstageCurrentHunk() tea.Cmd {
	idx := m.diffView.currentHunkIndex()
	if idx < 0 || m.diffView.file == nil {
		return nil
	}
	hunk := m.diffView.file.Hunks[idx]
	if hunk.Source != git.SourceStaged {
		m.statusMsg = "Can only unstage staged hunks"
		return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
	}
	path := m.diffView.file.Path
	status := m.diffView.file.Status
	return func() tea.Msg {
		err := git.UnstageHunk(path, status, hunk)
		return hunkApplyMsg{err: err}
	}
}

// discardCurrentHunk prompts for confirmation, then discards the hunk at the cursor.
func (m *Model) discardCurrentHunk() tea.Cmd {
	idx := m.diffView.currentHunkIndex()
	if idx < 0 || m.diffView.file == nil {
		return nil
	}
	hunk := m.diffView.file.Hunks[idx]
	if hunk.Source == git.SourceBranch {
		m.statusMsg = "Cannot discard committed changes"
		return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
	}
	path := m.diffView.file.Path
	status := m.diffView.file.Status
	m.confirmDiscard = true
	m.confirmMsg = "Discard hunk? This cannot be undone."
	m.pendingDiscardCmd = func() tea.Msg {
		err := git.DiscardHunk(path, status, hunk)
		return hunkApplyMsg{err: err}
	}
	return nil
}

// stageCurrentFile stages all unstaged changes in the current file.
func (m *Model) stageCurrentFile() tea.Cmd {
	file := m.diffView.file
	if file == nil {
		return nil
	}
	if !fileHasSource(file, git.SourceUnstaged) {
		m.statusMsg = "No unstaged changes to stage"
		return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
	}
	path := file.Path
	return func() tea.Msg {
		err := git.StageFile(path)
		return hunkApplyMsg{err: err}
	}
}

// unstageCurrentFile unstages all staged changes in the current file.
func (m *Model) unstageCurrentFile() tea.Cmd {
	file := m.diffView.file
	if file == nil {
		return nil
	}
	if !fileHasSource(file, git.SourceStaged) {
		m.statusMsg = "No staged changes to unstage"
		return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
	}
	path := file.Path
	return func() tea.Msg {
		err := git.UnstageFile(path)
		return hunkApplyMsg{err: err}
	}
}

// discardCurrentFile prompts for confirmation, then discards all changes in the current file.
func (m *Model) discardCurrentFile() tea.Cmd {
	file := m.diffView.file
	if file == nil {
		return nil
	}
	hasUnstaged := fileHasSource(file, git.SourceUnstaged)
	hasStaged := fileHasSource(file, git.SourceStaged)
	if !hasUnstaged && !hasStaged {
		m.statusMsg = "No changes to discard"
		return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
	}
	path := file.Path
	status := file.Status
	m.confirmDiscard = true
	m.confirmMsg = "Discard all changes to " + path + "?\nThis cannot be undone."
	m.pendingDiscardCmd = func() tea.Msg {
		err := git.DiscardFile(path, status, hasStaged)
		return hunkApplyMsg{err: err}
	}
	return nil
}

// openInEditor suspends the TUI and runs $VISUAL/$EDITOR on the current
// file at the cursor line. On return, an editorFinishedMsg triggers a
// diff reload so any edits show up immediately.
//
// When the cursor isn't on a code line (hunk header, comment display row,
// file with no diff), lineNum falls back to 0 and the editor opens the
// file at its top.
//
// Returns nil (no-op) if there's no file selected.
func (m *Model) openInEditor() tea.Cmd {
	file := m.diffView.file
	if file == nil {
		return nil
	}
	// Deleted files don't exist on disk — opening one in the editor
	// would either error out (VS Code) or create an empty buffer that
	// silently re-adds the file on save. Block with a hint instead.
	if file.Status == git.StatusDeleted {
		m.statusMsg = "Cannot edit deleted file"
		return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
	}
	filePath := file.Path
	// In git diff mode, file paths are repo-relative. Resolve to absolute
	// so the editor opens the right file even when revise was launched
	// from a subdirectory. In file review mode, the path is whatever the
	// user passed to `revise <file>` — use it as-is.
	if !m.fileReviewMode {
		if root, err := git.RepoRoot(); err == nil {
			filePath = filepath.Join(root, filePath)
		}
	}
	lineNum := 0
	if ref := m.diffView.cursorRef(); ref != nil {
		if ref.lineType == git.LineRemoved {
			lineNum = ref.oldNum
		} else {
			lineNum = ref.newNum
		}
	}
	cmd, err := editor.Command(filePath, lineNum)
	if err != nil {
		m.statusMsg = "Editor error: " + err.Error()
		return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

// fileHasSource returns true if any hunk in the file has the given source.
func fileHasSource(f *git.FileDiff, source git.HunkSource) bool {
	for _, h := range f.Hunks {
		if h.Source == source {
			return true
		}
	}
	return false
}

// exportComments copies comments to the clipboard or writes to a file.
// Returns a status string describing what happened, or "" if there's nothing to export.
func (m *Model) exportComments() string {
	text := formatExport(m.diff.Files, m.comments, m.marks)
	if text == "" {
		return ""
	}
	// Try common clipboard tools, fall back to writing a file
	for _, args := range [][]string{{"pbcopy"}, {"wl-copy"}, {"xclip", "-selection", "clipboard"}} {
		if _, err := exec.LookPath(args[0]); err != nil {
			continue
		}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if cmd.Run() == nil {
			return "Copied to clipboard"
		}
	}
	_ = os.WriteFile(".revise-comments.md", []byte(text), 0644)
	return "Saved to .revise-comments.md"
}

func (m Model) renderConfirmDialog() string {
	content := m.confirmMsg + "\n\n" +
		confirmKeyStyle.Render("y") + "/" + confirmKeyStyle.Render("Enter") + " confirm    " +
		"any key to cancel"

	return confirmStyle.Render(content)
}

// overlayCenter draws fg centered on top of bg, replacing the background characters.
func overlayCenter(bg, fg string, width, height int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	fgH := len(fgLines)
	fgW := 0
	for _, l := range fgLines {
		if w := ansi.StringWidth(l); w > fgW {
			fgW = w
		}
	}

	startY := (height - fgH) / 2
	startX := (width - fgW) / 2
	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	for i, fgLine := range fgLines {
		row := startY + i
		if row >= len(bgLines) {
			break
		}
		bgLine := bgLines[row]
		bgW := ansi.StringWidth(bgLine)

		var left string
		if startX > 0 && bgW > 0 {
			left = ansi.Truncate(bgLine, startX, "")
		}
		// Pad left to startX if background is shorter
		for ansi.StringWidth(left) < startX {
			left += " "
		}

		fgLineW := ansi.StringWidth(fgLine)
		rightStart := startX + fgLineW
		var right string
		if rightStart < bgW {
			// Cut the right portion from the background
			right = ansi.TruncateLeft(bgLine, rightStart, "")
		}

		bgLines[row] = left + fgLine + right
	}

	return strings.Join(bgLines, "\n")
}

func (m Model) mouseFocusDiff(msg tea.MouseMsg) bool {
	if m.fullscreen {
		return true
	}
	return msg.X > m.fileList.width+2
}

// sliderModeAt returns the DiffMode at the given X position in the slider,
// or -1 if the click is outside the slider labels. Branch is always included
// in the layout, even when disabled — callers should check availableModes()
// before acting on the result.
func (m Model) sliderModeAt(x int) DiffMode {
	type region struct {
		start, end int
		mode       DiffMode
	}
	var regions []region
	pos := 0

	labels := []struct {
		name string
		mode DiffMode
	}{
		{"Branch", ModeBranch},
		{"Staged", ModeStaged},
		{"Unstaged", ModeUnstaged},
	}
	for i, l := range labels {
		regions = append(regions, region{pos, pos + len(l.name) - 1, l.mode})
		pos += len(l.name)
		if i < len(labels)-1 {
			pos++ // separator
		}
	}

	for _, r := range regions {
		if x >= r.start && x <= r.end {
			return r.mode
		}
	}
	return -1
}

// availableModes returns the modes available for the current branch state.
// Order matches display: Branch · Staged · Unstaged (broadest → narrowest).
func (m Model) availableModes() []DiffMode {
	if m.onDefaultBranch {
		return []DiffMode{ModeStaged, ModeStagedOnly, ModeUnstaged}
	}
	return []DiffMode{ModeBranch, ModeStaged, ModeStagedOnly, ModeUnstaged}
}

// modeAvailable reports whether the given mode is selectable in the current
// branch state.
func (m Model) modeAvailable(mode DiffMode) bool {
	for _, am := range m.availableModes() {
		if am == mode {
			return true
		}
	}
	return false
}

// cycleMode advances the mode by direction (+1 or -1), wrapping around.
func (m *Model) cycleMode(direction int) {
	modes := m.availableModes()
	currentIdx := 0
	for i, mode := range modes {
		if mode == m.mode {
			currentIdx = i
			break
		}
	}
	nextIdx := (currentIdx + direction + len(modes)) % len(modes)
	m.mode = modes[nextIdx]
	m.modeExplicitlySet = true
}

// requestRefresh is the single funnel for "something says state may have
// changed — please poll" (FocusMsg, fsEventMsg, etc.). It bumps pollGen
// so any earlier scheduled tick is invalidated, then either fires a
// refreshTickMsg right away (if debounce permits) or schedules one for
// when debounce expires.
//
// Returns nil when an in-flight poll is already running — its result
// will reschedule the next tick, so a duplicate request is redundant.
func (m *Model) requestRefresh() tea.Cmd {
	if m.polling {
		return nil
	}
	m.pollGen++
	gen := m.pollGen
	now := time.Now()
	wait := m.refreshPolicy.Debounce(m.lastPollStart, now)
	if wait == 0 {
		m.nextTickAt = now
		return func() tea.Msg { return refreshTickMsg{gen: gen} }
	}
	m.nextTickAt = now.Add(wait)
	return tea.Tick(wait, func(time.Time) tea.Msg { return refreshTickMsg{gen: gen} })
}

// listenFSEvents returns a Cmd that blocks on the watcher's Events
// channel and emits one fsEventMsg per Event. The fsEventMsg handler is
// responsible for re-issuing this Cmd to keep the pump alive — and that
// is the only place that re-issues, to avoid double-listen races.
//
// Returns nil if the watcher is disabled (nil) so callers can safely
// batch unconditionally.
func listenFSEvents(w *fswatch.Watcher) tea.Cmd {
	if w == nil {
		return nil
	}
	ch := w.Events()
	return func() tea.Msg {
		_, ok := <-ch
		if !ok {
			return nil
		}
		return fsEventMsg{}
	}
}

// loadDiff returns a command that fetches the diff for the current mode.
func (m *Model) loadDiff() tea.Cmd {
	return m.loadDiffWithOptions(false)
}

// loadDiffFromPoll returns a loadDiff command tagged as originating from polling,
// so the UI can preserve scroll position.
func (m *Model) loadDiffFromPoll() tea.Cmd {
	return m.loadDiffWithOptions(true)
}

func (m *Model) loadDiffWithOptions(fromPoll bool) tea.Cmd {
	mode := m.mode
	ctx := m.contextLines
	hideWS := m.hideWhitespace
	m.lastDiffLoadStart = time.Now()
	start := m.lastDiffLoadStart
	return func() tea.Msg {
		// Re-check whether we're on the default branch so the UI can
		// enable/disable Branch mode dynamically (e.g. after git checkout -b).
		onDefault, _ := git.IsOnDefaultBranch()

		var diff *git.Diff
		var err error
		switch mode {
		case ModeBranch:
			diff, err = git.BranchDiffOptions(ctx, hideWS)
		case ModeStaged:
			diff, err = git.WorkingTreeDiffOptions(ctx, hideWS)
		case ModeStagedOnly:
			diff, err = git.StagedOnlyDiffOptions(ctx, hideWS)
		case ModeUnstaged:
			diff, err = git.UnstagedOnlyDiffOptions(ctx, hideWS)
		}
		return diffLoadedMsg{diff: diff, err: err, onDefaultBranch: onDefault, noCommits: !git.HasCommits(), fromPoll: fromPoll, duration: time.Since(start)}
	}
}

func (m *Model) syncSelectedFile() {
	m.diffView.setFile(m.fileList.selectedFile())
}

func (m *Model) nextFile() {
	m.fileList.moveDown()
	m.syncSelectedFile()
}

func (m *Model) prevFile() {
	m.fileList.moveUp()
	m.syncSelectedFile()
}

func (m Model) renderModeSlider() string {
	render := func(name string, active, disabled bool) string {
		switch {
		case disabled:
			return modeDisabledStyle.Render(name)
		case active:
			return modeActiveStyle.Render(name)
		default:
			return modeInactiveStyle.Render(name)
		}
	}

	type label struct {
		name     string
		active   bool
		disabled bool
	}

	// Cumulative: broadest mode (ModeBranch) lights all,
	// narrower modes drop components from the left.
	// ModeStagedOnly lights only Staged; ModeUnstaged lights only Unstaged.
	// Branch is always shown — greyed out on the default branch.
	labels := []label{
		{"Branch", m.mode == ModeBranch, m.onDefaultBranch},
		{"Staged", m.mode == ModeBranch || m.mode == ModeStaged || m.mode == ModeStagedOnly, false},
		{"Unstaged", m.mode == ModeBranch || m.mode == ModeStaged || m.mode == ModeUnstaged, false},
	}

	var result string
	for i, l := range labels {
		if i > 0 {
			if labels[i-1].active && l.active {
				result += modeActiveStyle.Render("+")
			} else {
				result += modeInactiveStyle.Render(" ")
			}
		}
		result += render(l.name, l.active, l.disabled)
	}

	return result
}

// renderDebugStrip is the --debug mode strip above the status bar. It
// surfaces refresh timings live so you can answer "is auto-refresh
// working?" / "why is the policy backing off so far?" without grepping
// logs. Empty string when not in --debug mode.
func (m Model) renderDebugStrip() string {
	if !m.debug {
		return ""
	}

	now := time.Now()

	field := func(label, value string) string {
		return devLabelStyle.Render(label+":") + " " + devValueStyle.Render(value)
	}

	fs := "off"
	if m.fsWatcher != nil {
		fs = "on"
	}
	focus := "blur"
	if m.focused {
		focus = "focus"
	}
	polling := "idle"
	if m.polling {
		polling = "in-flight"
	}

	// "next" shows what's actually going to happen, not just the
	// scheduled tick time: while polling, the tick is moot (a poll
	// is already running); while blurred, no tick will fire until
	// FocusMsg requests one. Without these overrides, a tick that
	// fires-and-drops (e.g. blurred) leaves nextTickAt in the past
	// and the field reads "now" forever.
	var nextDisplay string
	switch {
	case m.polling:
		nextDisplay = "polling…"
	case !m.focused:
		nextDisplay = "blurred"
	default:
		nextDisplay = untilOrDash(m.nextTickAt, now)
	}

	parts := []string{
		devLabelStyle.Render("[DEV]"),
		field("focus", focus),
		field("fs", fs),
		field("status", polling),
		field("poll", agoAndDur(m.lastPollStart, m.lastPollDuration, now)),
		field("diff", agoAndDur(m.lastDiffLoadStart, m.lastDiffLoadDuration, now)),
		field("next", nextDisplay),
		field("gen", fmt.Sprintf("%d", m.pollGen)),
	}

	return statusBarStyle.Width(m.width).Render(strings.Join(parts, "  "))
}

// agoAndDur formats "Xs ago (Yms)" — when an operation last started and
// how long it took. Returns "—" if the operation has never run.
func agoAndDur(start time.Time, dur time.Duration, now time.Time) string {
	if start.IsZero() {
		return "—"
	}
	return fmt.Sprintf("%s ago (%s)", shortDuration(now.Sub(start)), shortDuration(dur))
}

// untilOrDash formats "Xs" until a future time, or "now" if past, or "—"
// if zero. Used for the "next scheduled tick" indicator.
func untilOrDash(t, now time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := t.Sub(now)
	if d <= 0 {
		return "now"
	}
	return shortDuration(d)
}

// shortDuration is a compact form of d for the debug strip:
//   - sub-millisecond shows microseconds (e.g. "320µs")
//   - sub-second shows milliseconds (e.g. "45ms")
//   - sub-minute shows seconds with one decimal (e.g. "2.3s")
//   - longer shows minutes (e.g. "3m12s")
func shortDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Millisecond:
		return fmt.Sprintf("%dµs", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	default:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
}

func (m Model) renderStatusBar() string {
	if m.commentInputActive {
		hint := helpKeyStyle.Render("Enter") + statusBarStyle.Render(": save  ") +
			helpKeyStyle.Render("Esc") + statusBarStyle.Render(": cancel")
		return statusBarStyle.Width(m.width).Render(hint)
	}

	if m.statusMsg != "" {
		return statusBarStyle.Width(m.width).Render(m.statusMsg)
	}

	helpHint := helpKeyStyle.Render("?") + statusBarStyle.Render(" help")

	if m.noCommits {
		hint := statusBarStyle.Render("No commits yet — staged and untracked files only  ") + helpHint
		return statusBarStyle.Width(m.width).Render(hint)
	}

	count := len(m.comments)
	if count > 0 {
		hint := pluralize(count, "comment", "comments") +
			"  " + helpKeyStyle.Render("e") + statusBarStyle.Render(" export") +
			"  " + helpHint
		return statusBarStyle.Width(m.width).Render(hint)
	}
	return statusBarStyle.Width(m.width).Render(helpHint)
}

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	statusBar := m.renderStatusBar()
	debugStrip := m.renderDebugStrip()

	slider := m.renderModeSlider()
	if m.fileReviewMode {
		slider = m.fileReviewPath
	}

	bottomChunks := []string{statusBar}
	if debugStrip != "" {
		// Debug strip sits directly above the status bar. updateLayout()
		// shrinks panel height by one row when m.debug is true so the
		// debug strip doesn't overflow the screen.
		bottomChunks = []string{debugStrip, statusBar}
	}

	var screen string
	if m.fullscreen {
		panels := m.diffView.render(true, m.contextLines, m.hideWhitespace, slider)
		screen = lipgloss.JoinVertical(lipgloss.Left, append([]string{panels}, bottomChunks...)...)
	} else {
		left := m.fileList.render(m.focus == focusFileList, slider)
		right := m.diffView.render(m.focus == focusDiffView, m.contextLines, m.hideWhitespace)
		panels := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
		screen = lipgloss.JoinVertical(lipgloss.Left, append([]string{panels}, bottomChunks...)...)
	}

	if m.confirmDiscard || m.confirmClear {
		screen = overlayCenter(screen, m.renderConfirmDialog(), m.width, m.height)
	}

	if m.showHelp {
		var helpBindings []BindingGroup
		if m.fileReviewMode {
			helpBindings = FileReviewBindingGroups()
		}
		screen = overlayCenter(screen, renderHelp(m.width, m.height, m.helpScroll, helpBindings), m.width, m.height)
	}

	return screen
}

// ExportedComments returns the formatted comment text for printing on exit.
// Returns "" if there are no comments.
func (m Model) ExportedComments() string {
	return formatExport(m.diff.Files, m.comments, m.marks)
}

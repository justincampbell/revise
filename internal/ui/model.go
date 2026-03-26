package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	commentstore "github.com/justincampbell/revise/internal/comments"
	"github.com/justincampbell/revise/internal/git"
	"github.com/justincampbell/revise/internal/update"
)

type clearStatusMsg struct{}

// pollTickMsg triggers a periodic check for working tree changes.
type pollTickMsg struct{}

// pollResultMsg carries the result of a fingerprint check.
type pollResultMsg struct {
	fingerprint string
	err         error
}

const pollInterval = 30 * time.Second

// hunkApplyMsg is sent when an async hunk stage/unstage/discard completes.
type hunkApplyMsg struct {
	err error
}

// updateCheckMsg carries the result of a background update check.
type updateCheckMsg struct {
	info *update.UpdateInfo
	err  error
}

// updateAppliedMsg is sent when an update download+install completes.
type updateAppliedMsg struct {
	newVersion string
	err        error
}

// diffLoadedMsg is sent when an async diff load completes.
type diffLoadedMsg struct {
	diff            *git.Diff
	err             error
	onDefaultBranch bool
	fromPoll        bool // true when triggered by auto-refresh polling
}

// DiffMode represents which diff view is active.
// Modes are cumulative: Unstaged is always included,
// broader modes add Staged, then Branch.
type DiffMode int

const (
	ModeBranch    DiffMode = iota // committed + staged + unstaged + untracked (broadest, feature branch only)
	ModeStaged                    // staged + unstaged + untracked
	ModeStagedOnly                // staged only (no unstaged or untracked)
	ModeUnstaged                  // unstaged + untracked only (narrowest)
)

type focusPanel int

const (
	focusFileList focusPanel = iota
	focusDiffView
)

const fileListWidth = 30

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

	mode            DiffMode
	onDefaultBranch bool
	contextLines    int
	hideWhitespace  bool

	comments           comments
	storePath          string // path to JSON file for comment persistence
	commentInputActive bool
	commentTarget      commentKey
	statusMsg          string

	confirmDiscard    bool    // waiting for confirmation to discard
	confirmMsg        string  // message shown in the discard confirmation modal
	pendingDiscardCmd tea.Cmd // the discard command to run on confirmation

	confirmClear bool // waiting for confirmation to clear all comments

	lastFingerprint string // last seen git status fingerprint for auto-refresh
	polling         bool   // true while a fingerprint check is in flight
	focused         bool   // true when the terminal has focus; polling pauses on blur

	currentVersion string              // version string from build
	updateInfo     *update.UpdateInfo  // populated after update check
	updating       bool                // true while downloading update
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

	// Load persisted comments if a store path is configured.
	if storePath != "" {
		if loaded, err := commentstore.Load(storePath); err == nil {
			c = commentsFromStringMap(loaded)
		}
	}

	dv.comments = c
	fl.comments = c

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
		storePath:       storePath,
		mode:            mode,
		onDefaultBranch: onDefaultBranch,
		contextLines:    git.DefaultContextLines,
		lastFingerprint: initialFP,
		focused:         true,
		currentVersion:  version,
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.Tick(pollInterval, func(time.Time) tea.Msg { return pollTickMsg{} }),
	}
	if m.currentVersion != "" && update.IsSemver(m.currentVersion) {
		ver := m.currentVersion
		cmds = append(cmds, func() tea.Msg {
			info, err := update.CheckForUpdate(ver, false)
			return updateCheckMsg{info: info, err: err}
		})
	}
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

		case "right", "l":
			if m.focus == focusDiffView {
				m.fullscreen = !m.fullscreen
				m.updateLayout()
			} else {
				m.focus = focusDiffView
			}
			return m, nil

		case "left", "h":
			if m.fullscreen {
				m.fullscreen = false
				m.updateLayout()
			}
			m.focus = focusFileList
			return m, nil

		case "f":
			m.fullscreen = !m.fullscreen
			if m.fullscreen {
				m.focus = focusDiffView
			}
			m.updateLayout()
			return m, nil

		case "esc":
			if m.fullscreen {
				m.fullscreen = false
				m.updateLayout()
			}
			m.focus = focusFileList
			return m, nil

		case "tab":
			m.cycleMode(+1)
			return m, m.loadDiff()
		case "shift+tab":
			m.cycleMode(-1)
			return m, m.loadDiff()

		// File list navigation (always works)
		case "n":
			m.nextFile()
			return m, nil
		case "N":
			m.prevFile()
			return m, nil

		// Toggle hide whitespace
		case "w":
			m.hideWhitespace = !m.hideWhitespace
			return m, m.loadDiff()

		// Adjust context lines
		case "+", "=":
			m.contextLines++
			return m, m.loadDiff()
		case "-", "_":
			if m.contextLines > 0 {
				m.contextLines--
				return m, m.loadDiff()
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
			if len(m.comments) > 0 {
				n := len(m.comments)
				noun := "comments"
				if n == 1 {
					noun = "comment"
				}
				m.confirmClear = true
				m.confirmMsg = fmt.Sprintf("Clear all comments? (%d %s)", n, noun)
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
		case tea.MouseButtonLeft:
			if msg.Action != tea.MouseActionPress {
				return m, nil
			}
			// Check for status bar slider click
			if msg.Y == m.height-1 {
				if mode := m.sliderModeAt(msg.X); mode >= 0 && mode != m.mode {
					m.mode = mode
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
							m.startCommentInput()
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
		if msg.err != nil {
			m.statusMsg = "Error: " + msg.err.Error()
			return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
		}
		m.diff = msg.diff

		// Update default-branch status so Branch mode becomes
		// available (or unavailable) dynamically.
		m.onDefaultBranch = msg.onDefaultBranch
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
		return m, nil

	case hunkApplyMsg:
		if msg.err != nil {
			m.statusMsg = "Error: " + msg.err.Error()
			return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
		}
		return m, m.loadDiff()

	case clearStatusMsg:
		m.statusMsg = ""
		return m, nil

	case tea.FocusMsg:
		m.focused = true
		// Resume polling now that we have focus again.
		return m, tea.Tick(pollInterval, func(time.Time) tea.Msg { return pollTickMsg{} })

	case tea.BlurMsg:
		m.focused = false
		return m, nil

	case pollTickMsg:
		// Skip if unfocused or if a fingerprint check is already in
		// flight to avoid piling up git status processes (see #129).
		// On blur, polling stops entirely; FocusMsg restarts it.
		// The in-flight check's pollResultMsg will schedule the next tick.
		if !m.focused || m.polling {
			return m, nil
		}
		m.polling = true
		return m, func() tea.Msg {
			fp, err := git.StatusFingerprint()
			return pollResultMsg{fingerprint: fp, err: err}
		}

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

	case pollResultMsg:
		m.polling = false
		// Don't schedule the next tick if unfocused; FocusMsg will restart it.
		var nextTick tea.Cmd
		if m.focused {
			nextTick = tea.Tick(pollInterval, func(time.Time) tea.Msg { return pollTickMsg{} })
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
	m.diffView.width = m.width - listW - 5 // file list borders (2) + gap (1) + diff borders (2)
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
	case "d":
		m.deleteCommentAtCursor()
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
	_ = commentstore.Save(m.storePath, m.comments.toStringMap())
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
	viewH := m.diffView.viewHeight()
	minRoom := inputBoxHeight + 1 // rows needed below cursor
	if m.diffView.cursor > m.diffView.offset+viewH-minRoom {
		m.diffView.offset = m.diffView.cursor - viewH + minRoom
		if m.diffView.offset < 0 {
			m.diffView.offset = 0
		}
	}

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

func (m *Model) deleteCommentAtCursor() {
	ref := m.diffView.cursorRef()
	if ref == nil || m.diffView.file == nil {
		return
	}
	key := ref.commentKey(m.diffView.file.Path)
	delete(m.comments, key)
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
	text := formatExport(m.diff.Files, m.comments)
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
// or -1 if the click is outside the slider labels.
func (m Model) sliderModeAt(x int) DiffMode {
	type region struct {
		start, end int
		mode       DiffMode
	}
	var regions []region
	pos := 0

	if !m.onDefaultBranch {
		label := "Branch"
		regions = append(regions, region{pos, pos + len(label) - 1, ModeBranch})
		pos += len(label) + 1 // +1 for "·" separator
	}

	label := "Staged"
	regions = append(regions, region{pos, pos + len(label) - 1, ModeStaged})
	pos += len(label) + 1

	label = "Unstaged"
	regions = append(regions, region{pos, pos + len(label) - 1, ModeUnstaged})

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
		return diffLoadedMsg{diff: diff, err: err, onDefaultBranch: onDefault, fromPoll: fromPoll}
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
	render := func(name string, active bool) string {
		if active {
			return modeActiveStyle.Render(name)
		}
		return modeInactiveStyle.Render(name)
	}

	type label struct {
		name   string
		active bool
	}
	var labels []label

	// Cumulative: broadest mode (ModeBranch) lights all,
	// narrower modes drop components from the left.
	// ModeStagedOnly lights only Staged; ModeUnstaged lights only Unstaged.
	if !m.onDefaultBranch {
		labels = append(labels, label{"Branch", m.mode == ModeBranch})
	}
	labels = append(labels,
		label{"Staged", m.mode == ModeBranch || m.mode == ModeStaged || m.mode == ModeStagedOnly},
		label{"Unstaged", m.mode == ModeBranch || m.mode == ModeStaged || m.mode == ModeUnstaged},
	)

	var result string
	for i, l := range labels {
		if i > 0 {
			// Use + between two active labels, space otherwise
			if labels[i-1].active && l.active {
				result += modeActiveStyle.Render("+")
			} else {
				result += modeInactiveStyle.Render(" ")
			}
		}
		result += render(l.name, l.active)
	}

	return result
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

	var screen string
	if m.fullscreen {
		panels := m.diffView.render(true, m.contextLines, m.hideWhitespace, m.renderModeSlider())
		screen = lipgloss.JoinVertical(lipgloss.Left, panels, statusBar)
	} else {
		left := m.fileList.render(m.focus == focusFileList, m.renderModeSlider())
		right := m.diffView.render(m.focus == focusDiffView, m.contextLines, m.hideWhitespace)
		panels := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
		screen = lipgloss.JoinVertical(lipgloss.Left, panels, statusBar)
	}

	if m.confirmDiscard || m.confirmClear {
		screen = overlayCenter(screen, m.renderConfirmDialog(), m.width, m.height)
	}

	if m.showHelp {
		screen = overlayCenter(screen, renderHelp(m.width, m.height, m.helpScroll), m.width, m.height)
	}

	return screen
}

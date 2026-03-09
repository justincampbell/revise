package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	revisecomments "github.com/justincampbell/revise/internal/comments"
	"github.com/justincampbell/revise/internal/git"
	"github.com/justincampbell/revise/internal/mcp"
)

type clearStatusMsg struct{}

// diffRefreshMsg carries a freshly computed diff from a background goroutine.
type diffRefreshMsg struct{ diff *git.Diff }

// diffPollMsg triggers a background diff re-computation.
type diffPollMsg struct{}

// reviewFilePollMsg triggers a synchronous re-read of the review JSON file.
type reviewFilePollMsg struct{}

type focusPanel int

const (
	focusFileList focusPanel = iota
	focusDiffView
)

const fileListWidth = 30

// diffPollInterval is how often the TUI refreshes the diff from git.
const diffPollInterval = 3 * time.Second

// reviewFilePollInterval is how often the TUI re-reads the review JSON file
// to pick up agent replies and closed-comment status.
const reviewFilePollInterval = 2 * time.Second

type Model struct {
	diff     *git.Diff
	fileList fileListModel
	diffView diffViewModel
	focus    focusPanel
	showHelp   bool
	fullscreen bool
	width    int
	height   int
	ready    bool

	comments          comments
	commentInputActive bool
	commentTarget     commentKey
	statusMsg         string

	// Review file integration
	reviewFilePath string
	reviewFile     *revisecomments.ReviewFile

	// MCP server reference (optional; nil when no server is running)
	mcpServer    *mcp.Server
	mcpConnected bool
}

func New(diff *git.Diff) Model {
	fl := newFileListModel(diff.Files)
	dv := newDiffViewModel()

	c := make(comments)
	dv.comments = c
	fl.comments = c

	if len(diff.Files) > 0 {
		dv.setFile(&diff.Files[0])
	}

	// Derive the review file path (best-effort; silently disabled if git fails).
	reviewFilePath, _ := revisecomments.FilePath()

	return Model{
		diff:           diff,
		fileList:       fl,
		diffView:       dv,
		focus:          focusFileList,
		comments:       c,
		reviewFilePath: reviewFilePath,
	}
}

// SetMCPServer attaches an MCP server to the model so that submitting
// comments also fast-signals wait_for_review.
func (m *Model) SetMCPServer(s *mcp.Server) {
	m.mcpServer = s
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(diffPollInterval, func(time.Time) tea.Msg { return diffPollMsg{} }),
		tea.Tick(reviewFilePollInterval, func(time.Time) tea.Msg { return reviewFilePollMsg{} }),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.commentInputActive {
			return m.updateCommentInput(msg)
		}

		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "?":
			m.showHelp = !m.showHelp
			return m, nil

		case "right", "l":
			m.focus = focusDiffView
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

		// File list navigation (always works)
		case "n":
			m.nextFile()
			return m, nil
		case "N":
			m.prevFile()
			return m, nil

		// Submit comments to Claude Code via the review JSON file.
		case "s":
			statusMsg := m.submitComments()
			if statusMsg != "" {
				m.statusMsg = statusMsg
				return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })
			}
			return m, nil

		// Manually refresh the diff.
		case "r":
			return m, getRefreshedDiff()

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
			if !m.mouseFocusDiff(msg) {
				// border (1) + header (1) = 2 lines before file entries
				if !m.commentInputActive {
					idx := msg.Y - 2 + m.fileList.offset
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

	case clearStatusMsg:
		m.statusMsg = ""
		return m, nil

	case diffPollMsg:
		// Schedule next poll and kick off a background diff refresh.
		return m, tea.Batch(
			getRefreshedDiff(),
			tea.Tick(diffPollInterval, func(time.Time) tea.Msg { return diffPollMsg{} }),
		)

	case diffRefreshMsg:
		// Skip while comment input is active to avoid the cursor jumping.
		if msg.diff != nil && len(msg.diff.Files) > 0 && !m.commentInputActive {
			m.applyRefreshedDiff(msg.diff)
		}
		return m, nil

	case reviewFilePollMsg:
		// Skip while comment input is active to avoid disrupting the input box.
		if !m.commentInputActive {
			m.pollReviewFile()
		}
		return m, tea.Tick(reviewFilePollInterval, func(time.Time) tea.Msg { return reviewFilePollMsg{} })

	case mcp.ConnectedMsg:
		m.mcpConnected = msg.Connected
		return m, nil
	}

	return m, nil
}

// applyRefreshedDiff replaces the current diff, preserving the file selection
// and the diff view scroll position when the same file is still selected.
func (m *Model) applyRefreshedDiff(newDiff *git.Diff) {
	selectedPath := ""
	if sel := m.fileList.selectedFile(); sel != nil {
		selectedPath = sel.Path
	}

	m.diff = newDiff
	m.fileList.files = newDiff.Files

	// Restore selection by path if possible.
	sameFile := false
	if selectedPath != "" {
		for i, f := range newDiff.Files {
			if f.Path == selectedPath {
				m.fileList.cursor = i
				sameFile = true
				break
			}
		}
	}
	if m.fileList.cursor >= len(m.fileList.files) {
		m.fileList.cursor = 0
		sameFile = false
	}

	if sameFile {
		// Update file content but keep scroll position and cursor.
		if sel := m.fileList.selectedFile(); sel != nil {
			m.diffView.file = sel
			m.diffView.rebuildLinesPreservingCursor()
		}
	} else {
		m.syncSelectedFile()
	}
}

// pollReviewFile reads the review JSON file and updates the display if it has changed.
func (m *Model) pollReviewFile() {
	if m.reviewFilePath == "" {
		return
	}
	rf, err := revisecomments.Read(m.reviewFilePath)
	if err != nil || rf == nil {
		return
	}
	m.reviewFile = rf
	m.diffView.reviewFile = rf
	m.diffView.rebuildLinesPreservingCursor()
}

// getRefreshedDiff returns a tea.Cmd that re-runs git diff in the background.
func getRefreshedDiff() tea.Cmd {
	return func() tea.Msg {
		diff, err := git.GetDiff()
		if err != nil {
			return nil
		}
		return diffRefreshMsg{diff}
	}
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
	m.diffView.width = m.width - listW - 3 // gap
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
	case "c", "enter":
		if m.diffView.cursorRef() != nil {
			m.startCommentInput()
		}
	case "d":
		m.deleteCommentAtCursor()
	}
	return m, nil
}

func (m Model) updateCommentInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		text := strings.TrimSpace(m.diffView.textInput.Value())
		if text != "" {
			// Preserve existing ID if the comment is being edited.
			id := ""
			if existing, ok := m.comments[m.commentTarget]; ok {
				id = existing.ID
			}
			if id == "" {
				id = revisecomments.NewID()
			}
			m.comments[m.commentTarget] = commentEntry{ID: id, Body: text}
		} else {
			delete(m.comments, m.commentTarget)
		}
		m.commentInputActive = false
		m.diffView.commentInputActive = false
		m.diffView.textInput.Blur()
		m.diffView.rebuildLinesPreservingCursor()
	case "esc":
		m.commentInputActive = false
		m.diffView.commentInputActive = false
		m.diffView.textInput.Blur()
	default:
		var cmd tea.Cmd
		m.diffView.textInput, cmd = m.diffView.textInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) startCommentInput() {
	ref := m.diffView.cursorRef()
	if ref == nil || m.diffView.file == nil {
		return
	}
	key := ref.commentKey(m.diffView.file.Path)
	m.commentTarget = key

	body := ""
	if entry, ok := m.comments[key]; ok {
		body = entry.Body
	}
	m.diffView.textInput.SetValue(body)
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

func (m *Model) deleteCommentAtCursor() {
	ref := m.diffView.cursorRef()
	if ref == nil || m.diffView.file == nil {
		return
	}
	key := ref.commentKey(m.diffView.file.Path)
	delete(m.comments, key)
	m.diffView.rebuildLinesPreservingCursor()
}

// submitComments writes all current comments to the review JSON file with
// submitted=true, and signals the MCP server if one is attached.
// Returns a status message to display in the status bar.
func (m *Model) submitComments() string {
	if len(m.comments) == 0 {
		return "No comments to submit"
	}
	if m.reviewFilePath == "" {
		return "Error: could not determine review file path"
	}

	root, err := revisecomments.RepoRoot()
	if err != nil {
		return "Error: " + err.Error()
	}
	branch, err := revisecomments.CurrentBranch()
	if err != nil {
		return "Error: " + err.Error()
	}

	rf := &revisecomments.ReviewFile{
		Repo:      root,
		Branch:    branch,
		Submitted: true,
		Comments:  toReviewComments(m.comments),
	}

	if err := revisecomments.Write(m.reviewFilePath, rf); err != nil {
		return "Error writing review file: " + err.Error()
	}

	if m.mcpServer != nil {
		m.mcpServer.NotifySubmit()
	}

	return fmt.Sprintf("Submitted %d comment(s) ✓", len(m.comments))
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

func (m Model) mouseFocusDiff(msg tea.MouseMsg) bool {
	if m.fullscreen {
		return true
	}
	return msg.X > m.fileList.width+2
}

func (m *Model) syncSelectedFile() {
	f := m.fileList.selectedFile()
	if f != nil {
		m.diffView.setFile(f)
	}
}

func (m *Model) nextFile() {
	m.fileList.moveDown()
	m.syncSelectedFile()
}

func (m *Model) prevFile() {
	m.fileList.moveUp()
	m.syncSelectedFile()
}

func (m Model) renderStatusBar() string {
	if m.commentInputActive {
		return statusBarStyle.Width(m.width).Render("Enter: save  Esc: cancel")
	}

	if m.statusMsg != "" {
		return statusBarStyle.Width(m.width).Render(m.statusMsg)
	}

	// Show comment text when cursor is on a commented line
	if m.focus == focusDiffView {
		ref := m.diffView.cursorRef()
		if ref != nil && m.diffView.file != nil {
			key := ref.commentKey(m.diffView.file.Path)
			if entry, ok := m.comments[key]; ok {
				return statusBarStyle.Width(m.width).Render("◆ " + entry.Body)
			}
		}
	}

	count := len(m.comments)

	var left string
	if count > 0 {
		left = fmt.Sprintf("%d comment(s) — c: add/edit  d: delete  s: submit  e: export", count)
	} else {
		left = "c: add comment  s: submit  e: export  r: refresh  ?: help  q: quit"
	}

	if m.mcpConnected {
		right := mcpConnectedStyle.Render("⚡ Claude connected")
		gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
		if gap < 1 {
			gap = 1
		}
		return statusBarStyle.Render(left + strings.Repeat(" ", gap) + right)
	}
	return statusBarStyle.Width(m.width).Render(left)
}

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	if m.showHelp {
		return renderHelp(m.width, m.height)
	}

	if len(m.diff.Files) == 0 {
		return lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			"No changes found",
		)
	}

	statusBar := m.renderStatusBar()

	if m.fullscreen {
		panels := m.diffView.render(true)
		return lipgloss.JoinVertical(lipgloss.Left, panels, statusBar)
	}

	left := m.fileList.render(m.focus == focusFileList)
	right := m.diffView.render(m.focus == focusDiffView)
	panels := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	return lipgloss.JoinVertical(lipgloss.Left, panels, statusBar)
}

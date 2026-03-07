package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justincampbell/revise/internal/git"
)

type focusPanel int

const (
	focusFileList focusPanel = iota
	focusDiffView
)

const fileListWidth = 30

type Model struct {
	diff     *git.Diff
	fileList fileListModel
	diffView diffViewModel
	focus      focusPanel
	showHelp   bool
	fullscreen bool
	width    int
	height   int
	ready    bool
}

func New(diff *git.Diff) Model {
	fl := newFileListModel(diff.Files)
	dv := newDiffViewModel()

	if len(diff.Files) > 0 {
		dv.setFile(&diff.Files[0])
	}

	return Model{
		diff:     diff,
		fileList: fl,
		diffView: dv,
		focus:    focusFileList,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
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
				idx := msg.Y - 2 + m.fileList.offset
				if idx >= 0 && idx < len(m.fileList.files) {
					m.fileList.cursor = idx
					m.syncSelectedFile()
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
	}

	return m, nil
}

func (m *Model) updateLayout() {
	panelH := m.height - 2 // leave room for status

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
		m.diffView.scrollDown(1)
	case "k", "up":
		m.diffView.scrollUp(1)
	case "g", "home":
		m.diffView.goToTop()
	case "G", "end":
		m.diffView.goToBottom()
	case "pgdown", " ":
		m.diffView.pageDown()
	case "pgup":
		m.diffView.pageUp()
	}
	return m, nil
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

	if m.fullscreen {
		return m.diffView.render(true)
	}

	left := m.fileList.render(m.focus == focusFileList)
	right := m.diffView.render(m.focus == focusDiffView)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	commentstore "github.com/justincampbell/revise/internal/comments"
)

// Verdict is the reviewer's overall decision, chosen on quit when --verdict is set.
type Verdict string

const (
	VerdictNone           Verdict = "none"
	VerdictApprove        Verdict = "approve"
	VerdictRequestChanges Verdict = "request-changes"
)

func (v Verdict) label() string {
	switch v {
	case VerdictApprove:
		return "Approve"
	case VerdictRequestChanges:
		return "Request changes"
	}
	return "No verdict"
}

// WithBrief shows a markdown context panel above the panes. It starts expanded.
func (m Model) WithBrief(text string) Model {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	m.brief = lines
	m.briefExpanded = len(lines) > 0
	m.updateLayout()
	return m
}

// WithVerdict makes q open a verdict prompt, and makes the export always start
// with a `verdict: <value>` line so callers can tell an approval from a stray quit.
func (m Model) WithVerdict() Model {
	m.reviewInput = newTextBox("Review comment (optional)", 500)
	m.verdictEnabled = true
	m.verdict = VerdictNone
	return m
}

func briefMaxLines(height int) int {
	return max(3, (height-3)/2)
}

func (m Model) briefContentWidth() int {
	return max(10, m.width-4) // border (2) + padding (2)
}

func (m Model) briefLines() []string {
	return wrapBrief(m.brief, m.briefContentWidth())
}

func wrapBrief(brief []string, width int) []string {
	var out []string
	for _, line := range brief {
		styled, _ := highlightLine(line, "brief.md", nil, 0, "")
		out = append(out, strings.Split(ansi.Wrap(styled, width, ""), "\n")...)
	}
	return out
}

func (m Model) briefMaxScroll() int {
	return max(0, len(m.briefLines())-briefMaxLines(m.height))
}

func (m *Model) scrollBrief(delta int) {
	m.briefScroll = min(max(0, m.briefScroll+delta), m.briefMaxScroll())
}

func (m Model) briefHeight() int {
	if len(m.brief) == 0 {
		return 0
	}
	if !m.briefExpanded {
		return 1
	}
	return min(len(m.briefLines()), briefMaxLines(m.height)) + 2
}

func (m *Model) toggleBrief() {
	if len(m.brief) == 0 {
		return
	}
	m.briefExpanded = !m.briefExpanded
	m.updateLayout()
}

func briefTitle(lines []string) string {
	for _, l := range lines {
		if t := strings.TrimSpace(strings.TrimLeft(l, "# ")); t != "" {
			return t
		}
	}
	return ""
}

func (m Model) renderBrief() string {
	if len(m.brief) == 0 {
		return ""
	}
	if !m.briefExpanded {
		label := statusBarStyle.Render("▸ Brief: ")
		hint := "  " + helpKeyStyle.Render("b") + statusBarStyle.Render(" expand")
		room := max(0, m.width-ansi.StringWidth(label)-ansi.StringWidth(hint))
		title := ansi.Truncate(briefTitle(m.brief), room, "…")
		return ansi.Truncate(label+helpTextStyle.Render(title)+hint, m.width, "")
	}

	lines := m.briefLines()
	maxLines := briefMaxLines(m.height)
	top := min(m.briefScroll, max(0, len(lines)-maxLines))
	lines = lines[top:]
	if len(lines) > maxLines {
		hidden := len(lines) - maxLines + 1
		more := statusBarStyle.Render(fmt.Sprintf("… +%d more lines  ", hidden)) +
			helpKeyStyle.Render("B") + statusBarStyle.Render(" read all")
		lines = append(lines[:maxLines-1], more)
	}
	box := briefStyle.Width(m.width - 2).Render(strings.Join(lines, "\n"))
	return overlayBorderTitle(box, " Brief · b collapse ")
}

func (m Model) briefReaderWidth() int {
	return max(10, m.width-4) // border (2) + padding (2)
}

// briefReaderRows is how many brief lines fit in the full-screen reader: the screen minus its border and footer.
func (m Model) briefReaderRows() int {
	return max(1, m.height-3)
}

func (m Model) briefReaderMaxScroll() int {
	return max(0, len(wrapBrief(m.brief, m.briefReaderWidth()))-m.briefReaderRows())
}

func (m *Model) scrollBriefReader(delta int) {
	m.briefReadScroll = min(max(0, m.briefReadScroll+delta), m.briefReaderMaxScroll())
}

func (m *Model) openBriefReader() {
	if len(m.brief) == 0 {
		return
	}
	m.briefReading = true
	m.briefReadScroll = 0
}

func (m Model) updateBriefReader(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	page := m.briefReaderRows()
	switch msg.String() {
	case "j", "down":
		m.scrollBriefReader(1)
	case "k", "up":
		m.scrollBriefReader(-1)
	case "pgdown", " ", "d", "ctrl+d":
		m.scrollBriefReader(page)
	case "pgup", "u", "ctrl+u":
		m.scrollBriefReader(-page)
	case "g", "home":
		m.briefReadScroll = 0
	case "G", "end":
		m.briefReadScroll = m.briefReaderMaxScroll()
	case "esc", "q", "B":
		m.briefReading = false
	case "ctrl+c":
		m.verdict = VerdictNone
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) renderBriefReader() string {
	lines := wrapBrief(m.brief, m.briefReaderWidth())
	rows := m.briefReaderRows()
	top := min(m.briefReadScroll, max(0, len(lines)-rows))
	end := min(len(lines), top+rows)
	body := strings.Join(lines[top:end], "\n")
	footer := helpKeyStyle.Render("j/k") + statusBarStyle.Render(" scroll  ") +
		helpKeyStyle.Render("g/G") + statusBarStyle.Render(" top/bottom  ") +
		helpKeyStyle.Render("Esc") + statusBarStyle.Render(" close  ") +
		statusBarStyle.Render(fmt.Sprintf("%d–%d of %d", top+1, end, len(lines)))
	box := verdictStyle.Padding(0, 1).Width(m.briefReaderWidth() + 2).
		Render(body + strings.Repeat("\n", rows-(end-top)) + "\n" + footer)
	return overlayBorderTitle(box, " Brief ")
}

// overlayBorderTitle writes title into the top border, two columns in.
func overlayBorderTitle(box, title string) string {
	first, rest, ok := strings.Cut(box, "\n")
	if !ok || ansi.StringWidth(first) < ansi.StringWidth(title)+4 {
		return box
	}
	left := ansi.Truncate(first, 2, "")
	right := ansi.TruncateLeft(first, 2+ansi.StringWidth(title), "")
	return left + briefTitleStyle.Render(title) + right + "\n" + rest
}

func (m Model) updateVerdictPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "a", "A":
		return m.startReviewInput(VerdictApprove)
	case "r", "R":
		return m.startReviewInput(VerdictRequestChanges)
	case "q", "ctrl+c":
		m.verdict = VerdictNone
		return m, tea.Quit
	case "esc":
		m.verdictPrompt = false
	}
	return m, nil
}

func (m Model) startReviewInput(v Verdict) (tea.Model, tea.Cmd) {
	m.verdict = v
	m.reviewInputActive = true
	m.reviewInput.SetValue(m.reviewComment)
	m.reviewInput.CursorEnd()
	m.fitReviewInput()
	return m, m.reviewInput.Focus()
}

func (m *Model) fitReviewInput() {
	fitTextBox(&m.reviewInput, min(60, max(20, m.width-16)), max(1, min(10, m.height-10)))
}

func (m Model) updateReviewInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	msg = asTyped(msg)
	switch msg.String() {
	case "enter":
		m.reviewComment = strings.TrimSpace(m.reviewInput.Value())
		m.reviewInputActive = false
		m.reviewInput.Blur()
		// A finished review shouldn't resurface on the next run; a "none" exit keeps the store so it can be resumed.
		_ = commentstore.Delete(m.storePath)
		return m, tea.Quit
	case "esc":
		m.reviewInputActive = false
		m.reviewInput.Blur()
		m.verdict = VerdictNone
		return m, nil
	case "ctrl+c":
		m.verdict = VerdictNone
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.reviewInput, cmd = m.reviewInput.Update(msg)
	m.fitReviewInput()
	return m, cmd
}

func (m Model) renderVerdictDialog() string {
	if m.reviewInputActive {
		content := helpGroupStyle.Render(m.verdict.label()) + "\n\n" +
			m.reviewInput.View() + "\n\n" +
			helpKeyStyle.Render("Enter") + " finish    " + helpKeyStyle.Render("Esc") + " back"
		return verdictStyle.Render(content)
	}
	row := func(key, desc string) string {
		return helpKeyStyle.Render(padRight(key, 6)) + helpTextStyle.Render(desc)
	}
	content := helpGroupStyle.Render("Finish review") + "\n\n" +
		row("a", "Approve") + "\n" +
		row("r", "Request changes") + "\n" +
		row("q", "Quit without a verdict") + "\n" +
		row("Esc", "Keep reviewing")
	return verdictStyle.Render(content)
}

func formatReviewExport(v Verdict, review, comments string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "verdict: %s\n", v)
	if review != "" {
		fmt.Fprintf(&sb, "\n# Review\n\n> %s\n", review)
	}
	if comments != "" {
		sb.WriteString("\n" + comments)
	}
	return sb.String()
}

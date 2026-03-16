package ui

// Binding describes a keyboard shortcut for display in help.
type Binding struct {
	Key     string
	Desc    string
	GitOnly bool // true if this binding only applies to git diff mode
}

// BindingGroup is a named group of keyboard shortcuts.
type BindingGroup struct {
	Name     string
	Bindings []Binding
}

// allBindings is the single source of truth for all keyboard shortcuts.
// The help overlay and --help output are generated from this list.
var allBindings = []BindingGroup{
	{"General", []Binding{
		{"← (h)", "Focus file list", false},
		{"→ (l)", "Focus diff view", false},
		{"n/N", "Next/prev file", false},
		{"Tab/S-Tab", "Cycle diff mode", true},
		{"f", "Toggle fullscreen diff", false},
		{"Esc", "Back to file list", false},
		{"?", "Toggle help", false},
		{"q", "Quit", false},
	}},
	{"File List", []Binding{
		{"j/k, ↑/↓", "Navigate files", false},
		{"Enter", "Select file and focus diff", false},
		{"s", "Stage file", true},
		{"u", "Unstage file", true},
		{"D", "Discard file", true},
	}},
	{"Diff View", []Binding{
		{"j/k, ↑/↓", "Move cursor", false},
		{"}/{ (]/[)", "Next/prev hunk", false},
		{"+/-", "More/fewer context lines", true},
		{"w", "Toggle hide whitespace", false},
		{"g/G", "Top/bottom", false},
		{"Fn+↓/↑", "Page down/up", false},
		{"Enter/c", "Add/edit comment on line", false},
		{"d", "Delete comment on line", false},
		{"s/S", "Stage hunk/file", true},
		{"u/U", "Unstage hunk/file", true},
		{"D", "Discard hunk", true},
	}},
	{"Global", []Binding{
		{"e", "Export comments to clipboard", false},
		{"!", "Report issue on GitHub", false},
	}},
	{"Comment Input", []Binding{
		{"Enter", "Save comment", false},
		{"Esc", "Cancel", false},
	}},
}

// BindingGroups returns all keyboard shortcut groups.
func BindingGroups() []BindingGroup {
	return allBindings
}

// FileReviewBindingGroups returns binding groups filtered for file review mode.
func FileReviewBindingGroups() []BindingGroup {
	var groups []BindingGroup
	for _, g := range allBindings {
		var bindings []Binding
		for _, b := range g.Bindings {
			if !b.GitOnly {
				bindings = append(bindings, b)
			}
		}
		if len(bindings) > 0 {
			groups = append(groups, BindingGroup{Name: g.Name, Bindings: bindings})
		}
	}
	return groups
}

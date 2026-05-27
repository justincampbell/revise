package ui

// Binding describes a keyboard shortcut for display in help.
type Binding struct {
	Key     string
	Desc    string
	GitOnly bool // true for bindings that only apply in git diff mode
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
		{Key: "← (h)", Desc: "Focus file list", GitOnly: true},
		{Key: "→ (l)", Desc: "Focus diff view", GitOnly: true},
		{Key: "n/N", Desc: "Next/prev file", GitOnly: true},
		{Key: "Tab/S-Tab", Desc: "Cycle diff mode", GitOnly: true},
		{Key: "f", Desc: "Toggle fullscreen diff", GitOnly: true},
		{Key: "Esc", Desc: "Back to file list", GitOnly: true},
		{Key: "r", Desc: "Force refresh diff", GitOnly: true},
		{Key: "?", Desc: "Toggle help"},
		{Key: "q", Desc: "Quit"},
	}},
	{"File List", []Binding{
		{Key: "j/k, ↑/↓", Desc: "Navigate files", GitOnly: true},
		{Key: "Enter", Desc: "Select file and focus diff", GitOnly: true},
		{Key: "c", Desc: "Add/edit file comment", GitOnly: true},
		{Key: "d", Desc: "Delete file comment", GitOnly: true},
		{Key: "s", Desc: "Stage file", GitOnly: true},
		{Key: "u", Desc: "Unstage file", GitOnly: true},
		{Key: "D", Desc: "Discard file", GitOnly: true},
	}},
	{"Diff View", []Binding{
		{Key: "←/→ (h/l)", Desc: "Scroll horizontally"},
		{Key: "j/k, ↑/↓", Desc: "Move cursor"},
		{Key: "}/{ (]/[)", Desc: "Next/prev hunk"},
		{Key: "+/-", Desc: "More/fewer context lines", GitOnly: true},
		{Key: "w", Desc: "Toggle hide whitespace", GitOnly: true},
		{Key: "g/G", Desc: "Top/bottom"},
		{Key: "Fn+↓/↑", Desc: "Page down/up"},
		{Key: "Enter/c", Desc: "Add/edit comment on line"},
		{Key: "m", Desc: "Toggle mark on line"},
		{Key: "d", Desc: "Delete comment/mark on line"},
		{Key: "s/S", Desc: "Stage hunk/file", GitOnly: true},
		{Key: "u/U", Desc: "Unstage hunk/file", GitOnly: true},
		{Key: "D", Desc: "Discard hunk", GitOnly: true},
	}},
	{"Global", []Binding{
		{Key: "e", Desc: "Export comments to clipboard"},
		{Key: "E", Desc: "Open current file at cursor line in $VISUAL/$EDITOR"},
		{Key: "C", Desc: "Clear all comments"},
		{Key: "Ctrl+U", Desc: "Apply available update", GitOnly: true},
		{Key: "!", Desc: "Report issue on GitHub"},
	}},
	{"Comment Input", []Binding{
		{Key: "Enter", Desc: "Save comment"},
		{Key: "Esc", Desc: "Cancel"},
	}},
}

// BindingGroups returns all keyboard shortcut groups.
func BindingGroups() []BindingGroup {
	return allBindings
}

// FileReviewBindingGroups returns binding groups with git-only bindings filtered out.
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

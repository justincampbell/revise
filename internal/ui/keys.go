package ui

// Binding describes a keyboard shortcut for display in help.
type Binding struct {
	Key  string
	Desc string
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
		{"← (h)", "Focus file list"},
		{"→ (l)", "Focus diff view"},
		{"n/N", "Next/prev file"},
		{"f", "Toggle fullscreen diff"},
		{"Esc", "Back to file list"},
		{"?", "Toggle help"},
		{"q", "Quit"},
	}},
	{"File List", []Binding{
		{"j/k, ↑/↓", "Navigate files"},
		{"Enter", "Select file and focus diff"},
	}},
	{"Diff View", []Binding{
		{"j/k, ↑/↓", "Move cursor"},
		{"g/G", "Top/bottom"},
		{"Fn+↓/↑", "Page down/up"},
		{"Enter/c", "Add/edit comment on line"},
		{"d", "Delete comment on line"},
	}},
	{"Global", []Binding{
		{"e", "Export comments to clipboard"},
	}},
	{"Comment Input", []Binding{
		{"Enter", "Save comment"},
		{"Esc", "Cancel"},
	}},
}

// BindingGroups returns all keyboard shortcut groups.
func BindingGroups() []BindingGroup {
	return allBindings
}

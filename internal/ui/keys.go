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
		{"Tab/S-Tab", "Cycle diff mode"},
		{"t", "Toggle tree view"},
		{"f", "Toggle fullscreen diff"},
		{"Esc", "Back to file list"},
		{"?", "Toggle help"},
		{"q", "Quit"},
	}},
	{"File List", []Binding{
		{"j/k, ↑/↓", "Navigate files"},
		{"Enter", "Select file and focus diff"},
		{"c", "Add/edit file comment"},
		{"d", "Delete file comment"},
		{"s", "Stage file"},
		{"u", "Unstage file"},
		{"D", "Discard file"},
	}},
	{"Diff View", []Binding{
		{"j/k, ↑/↓", "Move cursor"},
		{"}/{ (]/[)", "Next/prev hunk"},
		{"+/-", "More/fewer context lines"},
		{"w", "Toggle hide whitespace"},
		{"g/G", "Top/bottom"},
		{"Fn+↓/↑", "Page down/up"},
		{"Enter/c", "Add/edit comment on line"},
		{"d", "Delete comment on line"},
		{"s/S", "Stage hunk/file"},
		{"u/U", "Unstage hunk/file"},
		{"D", "Discard hunk"},
	}},
	{"Global", []Binding{
		{"e", "Export comments to clipboard"},
		{"C", "Clear all comments"},
		{"Ctrl+U", "Apply available update"},
		{"!", "Report issue on GitHub"},
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

package git

// Diff represents the complete diff output.
type Diff struct {
	Files []FileDiff
}

// FileStatus represents the type of change.
type FileStatus string

const (
	StatusModified FileStatus = "M"
	StatusAdded    FileStatus = "A"
	StatusDeleted  FileStatus = "D"
	StatusRenamed  FileStatus = "R"
	StatusUntracked FileStatus = "?"
)

// FileDiff represents the diff for a single file.
type FileDiff struct {
	Path    string
	OldPath string // for renames
	Status  FileStatus
	Hunks   []Hunk
}

// Hunk represents a single diff hunk.
type Hunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Header   string // the @@ line
	Lines    []Line
}

// LineType represents the type of a diff line.
type LineType int

const (
	LineContext LineType = iota
	LineAdded
	LineRemoved
)

// Line represents a single line in a diff.
type Line struct {
	Type    LineType
	Content string
	OldNum  int // 0 if not applicable
	NewNum  int // 0 if not applicable
}

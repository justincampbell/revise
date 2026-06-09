package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	chromachroma "github.com/alecthomas/chroma/v2"
	chromalexers "github.com/alecthomas/chroma/v2/lexers"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
	"github.com/justincampbell/revise/internal/git"
	"github.com/stretchr/testify/assert"
)

func TestEscapeControlChars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"plain text", "hello", "hello"},
		{"tab preserved", "\t", "\t"},
		{"null byte", "\x00", "\u2400"},
		{"ESC", "\x1b", "\u241b"},
		{"DEL", "\x7f", "\u2421"},
		{"mixed", "a\x00b\tc", "a\u2400b\tc"},
		{"all control 0x01-0x1f except tab", "\x01\x09\x1f", "\u2401\t\u241f"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, escapeControlChars(tt.input))
		})
	}
}

func makeHunks(contents ...string) []git.Hunk {
	lines := make([]git.Line, len(contents))
	for i, c := range contents {
		lines[i] = git.Line{Type: git.LineContext, Content: c}
	}
	return []git.Hunk{{Lines: lines}}
}

func TestDetectIndentSize(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  int
	}{
		{"empty", []string{}, 0},
		{"no indentation", []string{"foo", "bar"}, 0},
		{"two spaces", []string{"  foo", "    bar"}, 2},
		{"four spaces", []string{"    foo"}, 4},
		{"tabs", []string{"\tfoo", "\t\tbar"}, 1},
		{"mixed — smallest wins", []string{"    foo", "  bar"}, 2},
		{"blank lines ignored", []string{"", "  foo"}, 2},
		{"single space", []string{" foo"}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, detectIndentSize(makeHunks(tt.lines...)))
		})
	}
}

func TestSplitLeadingWhitespace(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		indentSize int
		wantRest   string
	}{
		{"no indent", "foo", 2, "foo"},
		{"zero indentSize", "  foo", 0, "  foo"},
		{"two spaces one level", "  foo", 2, "foo"},
		{"four spaces two levels", "    foo", 2, "foo"},
		{"remainder spaces", "   foo", 2, "foo"},
		{"tabs", "\t\tfoo", 1, "foo"},
		{"empty string", "", 2, ""},
		{"only whitespace", "  ", 2, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, rest := splitLeadingWhitespace(tt.content, tt.indentSize, nil)
			assert.Equal(t, tt.wantRest, rest)
		})
	}

	t.Run("guides contain guide char for each level", func(t *testing.T) {
		guides, _ := splitLeadingWhitespace("    foo", 2, nil)
		count := strings.Count(guides, indentGuide)
		assert.Equal(t, 2, count)
	})

	t.Run("tab guides contain guide char for each tab", func(t *testing.T) {
		guides, _ := splitLeadingWhitespace("\t\tfoo", 1, nil)
		count := strings.Count(guides, indentGuide)
		assert.Equal(t, 2, count)
	})
}

func TestAddIndentGuides_NoColor(t *testing.T) {
	orig := noColor
	noColor = true
	defer func() { noColor = orig }()

	// noColor path should escape control chars but not add guides
	result := addIndentGuides("  \x1bfoo", 2, nil)
	assert.Equal(t, "  \u241bfoo", result)
	assert.NotContains(t, result, indentGuide)
}

func TestAddIndentGuides_WithColor(t *testing.T) {
	orig := noColor
	noColor = false
	defer func() { noColor = orig }()

	result := addIndentGuides("  foo", 2, nil)
	assert.Contains(t, result, indentGuide)
	assert.Contains(t, result, "foo")
}

func TestHighlightLine_CacheHit(t *testing.T) {
	orig := noColor
	noColor = false
	defer func() { noColor = orig }()

	clearHighlightCache()

	// First call — cache miss, should highlight a Go file
	result1, ok1 := highlightLine("package main", "main.go", nil, 0, "")
	assert.True(t, ok1)

	// Second call — should return cached result
	result2, ok2 := highlightLine("package main", "main.go", nil, 0, "")
	assert.True(t, ok2)
	assert.Equal(t, result1, result2)
}

func TestHighlightLine_NoColor(t *testing.T) {
	orig := noColor
	noColor = true
	defer func() { noColor = orig }()

	result, ok := highlightLine("package main", "main.go", nil, 0, "")
	assert.False(t, ok)
	assert.Equal(t, "package main", result)
}

func TestHighlightLine_UnknownExtension(t *testing.T) {
	orig := noColor
	noColor = false
	defer func() { noColor = orig }()

	result, ok := highlightLine("some content", "file.xyzunknown", nil, 0, "")
	assert.False(t, ok)
	assert.Equal(t, "some content", result)
}

func TestHighlightLine_CacheKeyIncludesTheme(t *testing.T) {
	orig := noColor
	noColor = false
	origTheme := activeTheme
	origIsDark := activeIsDark
	defer func() {
		noColor = orig
		activeTheme = origTheme
		activeIsDark = origIsDark
	}()

	clearHighlightCache()

	SetTheme(ThemeCharmtoneDark, true)
	bg1 := paletteFor(ThemeCharmtoneDark, true).addedBg
	result1, ok1 := highlightLine("package main", "main.go", bg1, 0, "")
	assert.True(t, ok1)

	SetTheme(ThemeGithubLight, false)
	bg2 := paletteFor(ThemeGithubLight, false).addedBg
	result2, ok2 := highlightLine("package main", "main.go", bg2, 0, "")
	assert.True(t, ok2)

	// Different themes + backgrounds produce different ANSI output
	assert.NotEqual(t, result1, result2)

	// After the second SetTheme call the cache was cleared, so only 1 entry remains
	highlightCacheMu.Lock()
	count := len(highlightCache)
	highlightCacheMu.Unlock()
	assert.Equal(t, 1, count)
}

func TestChromaFormatter_BakesBackground(t *testing.T) {
	bg := lipgloss.Color("#1C3634")
	f := chromaFormatter{bg: bg}

	// Use a real lexer from the registry
	lexer := chromachroma.Coalesce(chromalexers.Get("go"))
	it, err := lexer.Tokenise(nil, "package main")
	assert.NoError(t, err)

	style := chromastyles.Get("monokai")
	var sb strings.Builder
	err = f.Format(&sb, style, it)
	assert.NoError(t, err)
	// Output should be non-empty and contain ANSI escape sequences
	out := sb.String()
	assert.NotEmpty(t, out)
	assert.Contains(t, out, "\x1b[")
}

func TestRenderDiffLine_HighlightedVsFallback(t *testing.T) {
	orig := noColor
	noColor = false
	defer func() { noColor = orig }()

	p := paletteFor(ThemeCharmtoneDark, true)
	line := git.Line{Type: git.LineAdded, Content: "package main", NewNum: 1}

	// With a known Go file — should highlight
	withHighlight := renderDiffLine(line, false, 80, "main.go", p, 0, "", "")
	// With unknown extension — plain fallback
	withoutHighlight := renderDiffLine(line, false, 80, "file.xyzunknown", p, 0, "", "")

	assert.NotEmpty(t, withHighlight)
	assert.NotEmpty(t, withoutHighlight)
	// Highlighted output should differ (contains more ANSI sequences)
	assert.NotEqual(t, withHighlight, withoutHighlight)
}

func TestChromaFormatter_CommentFgOverride(t *testing.T) {
	lexer := chromachroma.Coalesce(chromalexers.Get("go"))
	style := chromastyles.Get("native")

	// Without commentFg — uses the style's default comment color (#999999)
	it1, _ := lexer.Tokenise(nil, "// a comment")
	var sb1 strings.Builder
	f1 := chromaFormatter{bg: nil}
	_ = f1.Format(&sb1, style, it1)
	without := sb1.String()

	// With commentFg — overrides to our color
	it2, _ := lexer.Tokenise(nil, "// a comment")
	var sb2 strings.Builder
	f2 := chromaFormatter{bg: nil, commentFg: lipgloss.Color("136")}
	_ = f2.Format(&sb2, style, it2)
	with := sb2.String()

	assert.NotEqual(t, without, with, "commentFg override should change output")
	// Override should use foreground ANSI code (38;5;136), not background
	assert.Contains(t, with, "38;5;136", "should contain foreground 256-color code")
	assert.NotContains(t, with, "48;5;136", "should not contain background 256-color code")
}

func TestChromaFormatter_CommentFgSkipsItalic(t *testing.T) {
	lexer := chromachroma.Coalesce(chromalexers.Get("go"))
	style := chromastyles.Get("native") // native uses italic comments

	it, _ := lexer.Tokenise(nil, "// a comment")
	var sb strings.Builder
	f := chromaFormatter{bg: nil, commentFg: lipgloss.Color("136")}
	_ = f.Format(&sb, style, it)
	result := sb.String()

	// Italic is ANSI code 3; should NOT be present when commentFg is set
	assert.NotContains(t, result, "\x1b[3;", "comment override should skip italic")
	assert.NotContains(t, result, ";3;", "comment override should skip italic")
}

func TestHighlightLine_AutoThemeCommentFg(t *testing.T) {
	orig := noColor
	noColor = false
	origTheme := activeTheme
	origIsDark := activeIsDark
	defer func() {
		noColor = orig
		activeTheme = origTheme
		activeIsDark = origIsDark
	}()

	SetTheme(ThemeAuto, true)

	result, ok := highlightLine("// a comment", "main.go", nil, 0, "")
	assert.True(t, ok)
	// Should use ANSI 256 color 136 (foreground), not italic
	assert.Contains(t, result, "38;5;136")
	assert.NotContains(t, result, "\x1b[3;")
}

func TestRenderDiffLine_MarkedSkipsHighlight(t *testing.T) {
	orig := noColor
	noColor = false
	defer func() { noColor = orig }()

	p := paletteFor(ThemeCharmtoneDark, true)
	line := git.Line{Type: git.LineAdded, Content: "package main", NewNum: 1}

	marked := renderDiffLine(line, true, 80, "main.go", p, 0, "", "")
	unmarked := renderDiffLine(line, false, 80, "main.go", p, 0, "", "")

	// Marked lines use mark styles, not syntax highlight styles — they differ
	assert.NotEqual(t, marked, unmarked)
}

func TestCodeFenceLang(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantFence bool
		wantLang  string
	}{
		{"not a fence", "regular text", false, ""},
		{"inline code only", "use `foo` here", false, ""},
		{"bare backtick fence", "```", true, ""},
		{"go fence", "```go", true, "go"},
		{"fence with space", "``` go", true, "go"},
		{"uppercase lang", "```Go", true, "go"},
		{"info string extra words", "```go title=main.go", true, "go"},
		{"tilde fence", "~~~", true, ""},
		{"tilde lang fence", "~~~python", true, "python"},
		{"indented fence", "   ```go", true, "go"},
		{"more than three backticks", "````go", true, "go"},
		{"backtick in info rejected", "```foo`bar", false, ""},
		{"empty string", "", false, ""},
		{"two backticks not a fence", "``not", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fence, lang := codeFenceLang(tt.content)
			assert.Equal(t, tt.wantFence, fence)
			assert.Equal(t, tt.wantLang, lang)
		})
	}
}

func TestIsMarkdownFile(t *testing.T) {
	assert.True(t, isMarkdownFile("README.md"))
	assert.True(t, isMarkdownFile("docs/guide.markdown"))
	assert.False(t, isMarkdownFile("main.go"))
	assert.False(t, isMarkdownFile("file.xyzunknown"))
}

func TestLexerByName(t *testing.T) {
	assert.NotNil(t, lexerByName("go"))
	assert.NotNil(t, lexerByName("GO"))        // case-insensitive
	assert.NotNil(t, lexerByName("js"))        // alias
	assert.Nil(t, lexerByName("totallybogus")) // unknown
	assert.Nil(t, lexerByName(""))             // empty
}

func TestHighlightLine_LangOverride(t *testing.T) {
	orig := noColor
	noColor = false
	defer func() { noColor = orig }()
	clearHighlightCache()

	// A line of Go code inside a Markdown file: without an override the markdown
	// lexer renders it as plain text; with the "go" override it's highlighted.
	const code = "func main() {}"
	asMarkdown, _ := highlightLine(code, "doc.md", nil, 0, "")
	asGo, ok := highlightLine(code, "doc.md", nil, 0, "go")
	assert.True(t, ok)
	assert.NotEqual(t, asMarkdown, asGo, "go override should change highlighting")
	assert.Contains(t, asGo, "\x1b[", "go override should emit ANSI styling")
}

func TestHighlightLine_LangOverrideUnknown(t *testing.T) {
	orig := noColor
	noColor = false
	defer func() { noColor = orig }()

	// Unknown language → no lexer → plain fallback.
	result, ok := highlightLine("some code", "doc.md", nil, 0, "totallybogus")
	assert.False(t, ok)
	assert.Equal(t, "some code", result)
}

func TestHighlightLine_LangOverrideInCacheKey(t *testing.T) {
	orig := noColor
	noColor = false
	defer func() { noColor = orig }()
	clearHighlightCache()

	// Same content + file but different overrides must not collide in the cache.
	plain, _ := highlightLine("x = 1", "doc.md", nil, 0, "")
	asPython, _ := highlightLine("x = 1", "doc.md", nil, 0, "python")
	assert.NotEqual(t, plain, asPython)

	// Re-fetching returns the per-override cached value, not a collision.
	plainAgain, _ := highlightLine("x = 1", "doc.md", nil, 0, "")
	assert.Equal(t, plain, plainAgain)
}

func TestHighlightLine_MarkdownHeadingBold(t *testing.T) {
	orig := noColor
	noColor = false
	origTheme := activeTheme
	origIsDark := activeIsDark
	defer func() {
		noColor = orig
		activeTheme = origTheme
		activeIsDark = origIsDark
	}()

	// Headings must render bold (ANSI code 1) but never underline (ANSI code 4)
	// across custom-entry themes, named-style themes ("github"), and the auto
	// theme's "native" style — which underlines heading tokens by default.
	for _, tc := range []struct {
		theme Theme
		dark  bool
	}{
		{ThemeGithubDark, true},   // custom entries
		{ThemeGithubLight, false}, // named "github" style
		{ThemeAuto, true},         // "native" style (underlines headings)
	} {
		for _, heading := range []string{"# Title", "## Subtitle"} {
			clearHighlightCache()
			SetTheme(tc.theme, tc.dark)
			result, ok := highlightLine(heading, "doc.md", nil, 0, "")
			assert.True(t, ok, "theme %s heading %q", tc.theme, heading)
			assert.Contains(t, result, "\x1b[1", "heading %q should be bold for theme %s", heading, tc.theme)
			assert.NotContains(t, result, "\x1b[4m", "heading %q should not be underlined for theme %s", heading, tc.theme)
		}
	}
}

func TestBuildLines_MarkdownFenceStateMachine(t *testing.T) {
	orig := noColor
	noColor = false
	origTheme := activeTheme
	origIsDark := activeIsDark
	defer func() {
		noColor = orig
		activeTheme = origTheme
		activeIsDark = origIsDark
	}()
	SetTheme(ThemeGithubDark, true)

	dv := diffViewModel{
		width: 80,
		file: &git.FileDiff{
			Path: "doc.md",
			Hunks: makeHunks(
				"# Heading",
				"",
				"```go",
				"func main() {}",
				"```",
				"func main() {}", // identical text, but outside the fence
			),
		},
	}
	dv.buildLines()

	// The code line inside the fence and the identical prose line outside it
	// must render differently: inside uses the Go lexer, outside uses Markdown.
	var rendered []string
	for i, ref := range dv.lineRefs {
		if ref != nil && ref.content == "func main() {}" {
			rendered = append(rendered, dv.lines[i])
		}
	}
	assert.Len(t, rendered, 2, "both occurrences should be present")
	assert.NotEqual(t, rendered[0], rendered[1],
		"fenced code should be highlighted as Go, prose as Markdown")

	// And the fenced line should match a direct Go-override render.
	p := paletteFor(activeTheme, activeIsDark)
	goLine := git.Line{Type: git.LineContext, Content: "func main() {}"}
	wantGo := renderDiffLine(goLine, false, dv.width-3, "doc.md", p, 0, "", "go")
	assert.Equal(t, wantGo, rendered[0], "fenced line should use the go override")
}

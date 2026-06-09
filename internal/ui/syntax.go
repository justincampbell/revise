package ui

import (
	"embed"
	"fmt"
	"image/color"
	"io"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/justincampbell/revise/internal/git"
)

//go:embed haml.xml
var hamlLexerFS embed.FS

func init() {
	lexers.Register(chroma.MustNewXMLLexer(hamlLexerFS, "haml.xml"))
}

// highlightCache caches highlighted lines keyed by theme+filePath+content.
// This avoids re-lexing the same line when switching back to a file.
// Cleared on theme change and when it exceeds highlightCacheMaxSize.
const highlightCacheMaxSize = 2000

var (
	highlightCache   = map[string]string{}
	highlightCacheMu sync.Mutex
)

// fileLexerCache caches the resolved lexer per filePath so lexers.Match isn't
// called on every line.
var (
	fileLexerCache   = map[string]chroma.Lexer{}
	fileLexerCacheMu sync.Mutex
)

// nameLexerCache caches the resolved lexer per language name (used for fenced
// Markdown code blocks, where the language comes from the fence info string
// rather than the file path).
var (
	nameLexerCache   = map[string]chroma.Lexer{}
	nameLexerCacheMu sync.Mutex
)

// clearHighlightCache discards all cached highlighted lines. Called by SetTheme
// so stale theme-keyed entries don't persist after a theme change.
func clearHighlightCache() {
	highlightCacheMu.Lock()
	highlightCache = map[string]string{}
	highlightCacheMu.Unlock()
}

// lexerFor returns the chroma lexer for the given file path, or nil if unknown.
// Results are cached to avoid iterating all lexer patterns per line.
func lexerFor(filePath string) chroma.Lexer {
	fileLexerCacheMu.Lock()
	defer fileLexerCacheMu.Unlock()
	if l, ok := fileLexerCache[filePath]; ok {
		return l
	}
	l := lexers.Match(filePath)
	if l != nil {
		l = chroma.Coalesce(l)
	}
	fileLexerCache[filePath] = l
	return l
}

// lexerByName returns the chroma lexer for a language name or alias (e.g. "go",
// "js"), or nil if unknown. Matching is case-insensitive. Results are cached.
func lexerByName(name string) chroma.Lexer {
	nameLexerCacheMu.Lock()
	defer nameLexerCacheMu.Unlock()
	if l, ok := nameLexerCache[name]; ok {
		return l
	}
	l := lexers.Get(name)
	if l != nil {
		l = chroma.Coalesce(l)
	}
	nameLexerCache[name] = l
	return l
}

// isMarkdownFile reports whether the file at filePath is highlighted by the
// Markdown lexer. Used to enable fenced-code-block detection.
func isMarkdownFile(filePath string) bool {
	l := lexerFor(filePath)
	return l != nil && l.Config().Name == "markdown"
}

// highlightLine applies chroma syntax highlighting to a single line of content,
// with the given background color baked into every token via a custom formatter.
// Returns the ANSI-colored string and true if highlighting was applied, or the
// original content and false if NO_COLOR is set or no lexer matches.
//
// When langOverride is non-empty, that language's lexer is used instead of the
// one inferred from filePath. This lets fenced Markdown code blocks be
// highlighted with their declared language (e.g. ```go).
func highlightLine(content, filePath string, bg color.Color, indentSize int, langOverride string) (string, bool) {
	if noColor {
		return content, false
	}

	var lexer chroma.Lexer
	if langOverride != "" {
		lexer = lexerByName(langOverride)
	} else {
		lexer = lexerFor(filePath)
	}
	if lexer == nil {
		return content, false
	}

	highlightCacheMu.Lock()
	theme := activeTheme
	isDark := activeIsDark
	bgKey := "nil"
	if bg != nil {
		r, g, b, a := bg.RGBA()
		bgKey = fmt.Sprintf("%d,%d,%d,%d", r, g, b, a)
	}
	cacheKey := string(theme) + "\x00" + filePath + "\x00" + langOverride + "\x00" + bgKey + "\x00" + content
	if cached, ok := highlightCache[cacheKey]; ok {
		highlightCacheMu.Unlock()
		return cached, true
	}
	highlightCacheMu.Unlock()

	var style *chroma.Style
	isAuto := theme == ThemeAuto || theme == ThemeAutoDaltonized
	if isAuto {
		// Auto themes use a built-in chroma style that works with the
		// terminal's native palette instead of hardcoded hex colors.
		if isDark {
			style = styles.Get("native")
		} else {
			style = styles.Get(chromaStyleFor())
		}
	} else if entries := chromaStyleEntries(theme); entries != nil {
		style = chroma.MustNewStyle("revise", entries)
	} else {
		style = styles.Get(chromaStyleFor())
	}

	// Strip leading whitespace before tokenising so chroma sees clean content,
	// then prepend the rendered indent guides.
	guides, stripped := splitLeadingWhitespace(content, indentSize, bg)

	// Append a newline so line-anchored lexer rules fire — the Markdown lexer
	// only recognises a heading (and similar constructs) when the line ends in a
	// newline. The formatter trims and skips the resulting empty token.
	it, err := lexer.Tokenise(nil, stripped+"\n")
	if err != nil {
		return content, false
	}

	var sb strings.Builder
	sb.WriteString(guides)
	f := chromaFormatter{bg: bg}
	if isAuto {
		f.commentFg = lipgloss.Color("136") // ANSI 256: dark yellow
	}
	if err := f.Format(&sb, style, it); err != nil {
		return content, false
	}

	result := sb.String()

	highlightCacheMu.Lock()
	if len(highlightCache) >= highlightCacheMaxSize {
		highlightCache = map[string]string{}
	}
	highlightCache[cacheKey] = result
	highlightCacheMu.Unlock()

	return result, true
}

// chromaFormatter is a custom chroma formatter that renders each token using
// lipgloss, with the diff line background baked into every token style. This
// prevents ANSI reset sequences from clearing the background mid-line.
//
// When commentFg is non-nil, comment tokens use that color instead of the
// style's hardcoded value — this lets auto mode use the terminal's own dim color.
type chromaFormatter struct {
	bg        color.Color
	commentFg color.Color
}

func (c chromaFormatter) Format(w io.Writer, style *chroma.Style, it chroma.Iterator) error {
	for token := it(); token != chroma.EOF; token = it() {
		value := escapeControlChars(strings.TrimRight(token.Value, "\n"))
		// Skip empty tokens (e.g. the appended trailing newline) so styling an
		// empty string doesn't emit stray escape sequences.
		if value == "" {
			continue
		}

		// Markdown headings (Generic.Heading / Generic.Subheading) should always
		// stand out as headings. Not every chroma style bolds them (the "github"
		// style, used for light themes, leaves them unstyled), so force bold here.
		isHeading := token.Type == chroma.GenericHeading || token.Type == chroma.GenericSubheading

		entry := style.Get(token.Type)
		if entry.IsZero() && !isHeading {
			_, _ = fmt.Fprint(w, value)
			continue
		}

		isComment := c.commentFg != nil && token.Type.InSubCategory(chroma.Comment)

		s := lipgloss.NewStyle()
		if c.bg != nil {
			s = s.Background(c.bg)
		}
		if entry.Bold == chroma.Yes || isHeading {
			s = s.Bold(true)
		}
		// Skip italic for overridden comments — some terminals render
		// italic text with a background highlight.
		if entry.Italic == chroma.Yes && !isComment {
			s = s.Italic(true)
		}
		if entry.Underline == chroma.Yes {
			s = s.Underline(true)
		}
		if isComment {
			s = s.Foreground(c.commentFg)
		} else if entry.Colour.IsSet() {
			s = s.Foreground(lipgloss.Color(entry.Colour.String()))
		}

		_, _ = fmt.Fprint(w, s.Render(value))
	}
	return nil
}

// indentGuide is the character used to mark indent levels.
const indentGuide = "▏"

// detectIndentSize scans hunks and returns the smallest non-zero indent unit
// found in line contents (in spaces). Tabs count as one unit. Returns 0 if no
// indented lines are found, which callers should treat as "no guides".
func detectIndentSize(hunks []git.Hunk) int {
	min := 0
	for _, hunk := range hunks {
		for _, line := range hunk.Lines {
			if len(line.Content) == 0 {
				continue
			}
			n := 0
			for _, ch := range line.Content {
				if ch == '\t' {
					n = 1
					break
				} else if ch == ' ' {
					n++
				} else {
					break
				}
			}
			if n > 0 && (min == 0 || n < min) {
				min = n
			}
		}
	}
	return min
}

// splitLeadingWhitespace extracts the leading whitespace from content, renders
// it as indent guides, and returns the rendered guides and the remaining
// non-whitespace content separately. This allows the non-whitespace portion to
// be passed to the syntax highlighter while guides are prepended after.
func splitLeadingWhitespace(content string, indentSize int, bg color.Color) (guides, rest string) {
	highlightCacheMu.Lock()
	guideColor := paletteFor(activeTheme, activeIsDark).dim
	highlightCacheMu.Unlock()
	return splitLeadingWhitespaceColor(content, indentSize, bg, guideColor)
}

func splitLeadingWhitespaceColor(content string, indentSize int, bg, guideColor color.Color) (guides, rest string) {
	if indentSize <= 0 || len(content) == 0 {
		return "", content
	}

	isTabs := content[0] == '\t'
	leading := 0
	for _, ch := range content {
		if isTabs && ch == '\t' {
			leading++
		} else if !isTabs && ch == ' ' {
			leading++
		} else {
			break
		}
	}
	if leading == 0 {
		return "", content
	}

	guideStyle := lipgloss.NewStyle().
		Foreground(guideColor).
		Background(bg)
	spaceStyle := lipgloss.NewStyle().Background(bg)

	var sb strings.Builder
	tabWidth := indentSize
	if tabWidth <= 1 {
		tabWidth = 2 // default tab display width
	}
	if isTabs {
		for i := 0; i < leading; i++ {
			sb.WriteString(guideStyle.Render(indentGuide))
			sb.WriteString(spaceStyle.Render(strings.Repeat(" ", tabWidth-1)))
		}
	} else {
		levels := leading / indentSize
		remainder := leading % indentSize
		for i := 0; i < levels; i++ {
			sb.WriteString(guideStyle.Render(indentGuide))
			if indentSize > 1 {
				sb.WriteString(spaceStyle.Render(strings.Repeat(" ", indentSize-1)))
			}
		}
		if remainder > 0 {
			sb.WriteString(spaceStyle.Render(strings.Repeat(" ", remainder)))
		}
	}

	return sb.String(), escapeControlChars(content[leading:])
}

// addIndentGuides renders indent guides for plain (non-highlighted) lines.
func addIndentGuides(content string, indentSize int, bg color.Color) string {
	if noColor {
		return escapeControlChars(content)
	}
	guides, rest := splitLeadingWhitespace(content, indentSize, bg)
	return guides + rest
}

// codeFenceLang reports whether a line is a Markdown fenced-code delimiter
// (``` or ~~~, optionally indented) and returns the info-string language.
// For an opening fence the language is the first token of the info string,
// lowercased (e.g. "go" for "```go"); it is "" for a bare fence (which may be
// either an opening fence with no language or a closing fence).
func codeFenceLang(content string) (isFence bool, lang string) {
	t := strings.TrimLeft(content, " ")
	var fenceChar byte
	switch {
	case strings.HasPrefix(t, "```"):
		fenceChar = '`'
	case strings.HasPrefix(t, "~~~"):
		fenceChar = '~'
	default:
		return false, ""
	}

	// Skip the run of fence characters.
	i := 0
	for i < len(t) && t[i] == fenceChar {
		i++
	}
	info := strings.TrimSpace(t[i:])

	// A backtick info string may not contain backticks (CommonMark): this avoids
	// treating inline code like `x = ``y`` ` as a fence.
	if fenceChar == '`' && strings.Contains(info, "`") {
		return false, ""
	}
	if info == "" {
		return true, ""
	}
	return true, strings.ToLower(strings.Fields(info)[0])
}

// escapeControlChars replaces control characters with Unicode Control Picture
// representations so they display safely in the terminal.
func escapeControlChars(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 0 && r <= 0x1f && r != '\t':
			sb.WriteRune('\u2400' + r)
		case r == ansi.DEL:
			sb.WriteRune('\u2421')
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

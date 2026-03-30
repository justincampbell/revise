package ui

import (
	"embed"
	"fmt"
	"image/color"
	"io"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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

// highlightLine applies chroma syntax highlighting to a single line of content,
// with the given background color baked into every token via a custom formatter.
// Returns the ANSI-colored string and true if highlighting was applied, or the
// original content and false if NO_COLOR is set or no lexer matches the file.
func highlightLine(content, filePath string, bg color.Color, indentSize int) (string, bool) {
	if noColor {
		return content, false
	}

	lexer := lexerFor(filePath)
	if lexer == nil {
		return content, false
	}

	highlightCacheMu.Lock()
	theme := activeTheme
	isDark := activeIsDark
	cacheKey := string(theme) + "\x00" + filePath + "\x00" + fmt.Sprintf("%v", bg) + "\x00" + content
	if cached, ok := highlightCache[cacheKey]; ok {
		highlightCacheMu.Unlock()
		return cached, true
	}
	highlightCacheMu.Unlock()

	var style *chroma.Style
	daltonized := theme == ThemeDarkDaltonized || theme == ThemeLightDaltonized
	if entries := chromaStyleEntries(isDark, daltonized); entries != nil {
		style = chroma.MustNewStyle("revise-dark", entries)
	} else {
		style = styles.Get(chromaStyleFor())
	}

	// Strip leading whitespace before tokenising so chroma sees clean content,
	// then prepend the rendered indent guides.
	guides, stripped := splitLeadingWhitespace(content, indentSize, bg)

	it, err := lexer.Tokenise(nil, stripped)
	if err != nil {
		return content, false
	}

	var sb strings.Builder
	sb.WriteString(guides)
	f := chromaFormatter{bg: bg}
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
type chromaFormatter struct {
	bg color.Color
}

func (c chromaFormatter) Format(w io.Writer, style *chroma.Style, it chroma.Iterator) error {
	for token := it(); token != chroma.EOF; token = it() {
		value := escapeControlChars(strings.TrimRight(token.Value, "\n"))

		entry := style.Get(token.Type)
		if entry.IsZero() {
			fmt.Fprint(w, value)
			continue
		}

		s := lipgloss.NewStyle().Background(c.bg)
		if entry.Bold == chroma.Yes {
			s = s.Bold(true)
		}
		if entry.Italic == chroma.Yes {
			s = s.Italic(true)
		}
		if entry.Underline == chroma.Yes {
			s = s.Underline(true)
		}
		if entry.Colour.IsSet() {
			s = s.Foreground(lipgloss.Color(entry.Colour.String()))
		}

		fmt.Fprint(w, s.Render(value))
	}
	return nil
}

// indentGuide is the character used to mark indent levels.
const indentGuide = "▏"

// detectIndentSize scans a slice of line contents and returns the smallest
// non-zero indent unit found (in spaces). Tabs count as one unit. Returns 0
// if no indented lines are found, which callers should treat as "no guides".
func detectIndentSize(lines []string) int {
	min := 0
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		n := 0
		for _, ch := range line {
			if ch == '\t' {
				// Tabs: treat each tab as one unit; report as 1 space-equivalent
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
	return min
}

// splitLeadingWhitespace extracts the leading whitespace from content, renders
// it as indent guides, and returns the rendered guides and the remaining
// non-whitespace content separately. This allows the non-whitespace portion to
// be passed to the syntax highlighter while guides are prepended after.
func splitLeadingWhitespace(content string, indentSize int, bg color.Color) (guides, rest string) {
	return splitLeadingWhitespaceColor(content, indentSize, bg, paletteFor(activeTheme, activeIsDark).dim)
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

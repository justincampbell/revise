package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
)

// Theme identifies the active color palette.
type Theme string

const (
	ThemeAuto                     Theme = "auto"
	ThemeAutoDaltonized           Theme = "auto-daltonized"
	ThemeCharmtoneDark            Theme = "charmtone-dark"
	ThemeCharmtoneDarkDaltonized  Theme = "charmtone-dark-daltonized"
	ThemeCharmtoneLight           Theme = "charmtone-light"
	ThemeCharmtoneLightDaltonized Theme = "charmtone-light-daltonized"
	ThemeGithubDark               Theme = "github-dark"
	ThemeGithubDarkDaltonized     Theme = "github-dark-daltonized"
	ThemeGithubLight              Theme = "github-light"
	ThemeGithubLightDaltonized    Theme = "github-light-daltonized"
)

// ValidThemes lists all accepted --theme values.
var ValidThemes = []Theme{
	ThemeAuto,
	ThemeAutoDaltonized,
	ThemeCharmtoneDark,
	ThemeCharmtoneDarkDaltonized,
	ThemeCharmtoneLight,
	ThemeCharmtoneLightDaltonized,
	ThemeGithubDark,
	ThemeGithubDarkDaltonized,
	ThemeGithubLight,
	ThemeGithubLightDaltonized,
}

// IsValidTheme reports whether t is a recognized theme name.
func IsValidTheme(t Theme) bool {
	for _, v := range ValidThemes {
		if t == v {
			return true
		}
	}
	return false
}

// IsDarkTheme reports whether a theme targets a dark terminal background.
func IsDarkTheme(t Theme) bool {
	switch t {
	case ThemeCharmtoneDark, ThemeCharmtoneDarkDaltonized,
		ThemeGithubDark, ThemeGithubDarkDaltonized:
		return true
	default:
		return false
	}
}

// chromaStyleFor returns the chroma style name for light themes.
// Dark themes use chromaStyleEntries() instead.
func chromaStyleFor() string {
	return "github"
}

// chromaStyleEntries returns a chroma.StyleEntries map for syntax highlighting.
// Returns nil when the named chroma style should be used directly.
// daltonized swaps green→blue for red-green colorblind users.
func chromaStyleEntries(t Theme) chroma.StyleEntries {
	switch t {
	case ThemeCharmtoneDark, ThemeCharmtoneDarkDaltonized:
		return charmtoneDarkChromaEntries(t == ThemeCharmtoneDarkDaltonized)
	case ThemeGithubDark, ThemeGithubDarkDaltonized:
		return githubDarkChromaEntries(t == ThemeGithubDarkDaltonized)
	case ThemeGithubLightDaltonized, ThemeCharmtoneLightDaltonized:
		return githubLightDaltonizedChromaEntries()
	default:
		return nil // use named style
	}
}

func charmtoneDarkChromaEntries(daltonized bool) chroma.StyleEntries {
	entries := chroma.StyleEntries{
		chroma.Background:          "bg:#3A3943",
		chroma.Text:                "#BFBCC8",
		chroma.Error:               "bg:#EB4268 #FFFAF1",
		chroma.Comment:             "#605F6B", // Oyster
		chroma.CommentPreproc:      "#FF6E63", // Bengal
		chroma.Keyword:             "#00A4FF", // Malibu
		chroma.KeywordReserved:     "#FF4FBF", // Pony
		chroma.KeywordNamespace:    "#FF4FBF", // Pony
		chroma.KeywordType:         "#7272FF", // Guppy
		chroma.Punctuation:         "#E8FE96", // Zest
		chroma.Name:                "#BFBCC8",
		chroma.NameBuiltin:         "#FF79D0", // Cheeky
		chroma.NameTag:             "#D46EFF", // Mauve
		chroma.NameAttribute:       "#8B75FF", // Hazy
		chroma.NameClass:           "bold underline #F1EFEF",
		chroma.NameConstant:        "#F1EFEF",
		chroma.NameDecorator:       "#E8FF27", // Citron
		chroma.NameOther:           "#BFBCC8",
		chroma.Literal:             "#BF976F", // Cumin
		chroma.LiteralString:       "#BF976F", // Cumin
		chroma.LiteralStringEscape: "#68FFD6", // Bok
		chroma.GenericEmph:         "italic",
		chroma.GenericStrong:       "bold",
		chroma.GenericSubheading:   "#858392", // Squid
	}
	if daltonized {
		entries[chroma.NameFunction] = "#4FBEFE"    // Sardine (blue)
		entries[chroma.Operator] = "#E8FE96"        // Zest (yellow)
		entries[chroma.GenericInserted] = "#4FBEFE" // Sardine (blue)
		entries[chroma.GenericDeleted] = "#FF985A"  // Tang (orange)
		entries[chroma.LiteralNumber] = "#4FBEFE"   // Sardine (blue)
	} else {
		entries[chroma.NameFunction] = "#12C78F"    // Guac (green)
		entries[chroma.Operator] = "#FF7F90"        // Salmon (pink)
		entries[chroma.GenericInserted] = "#12C78F" // Guac
		entries[chroma.GenericDeleted] = "#FF577D"  // Coral (red)
		entries[chroma.LiteralNumber] = "#00FFB2"   // Julep (green)
	}
	return entries
}

func githubDarkChromaEntries(daltonized bool) chroma.StyleEntries {
	entries := chroma.StyleEntries{
		chroma.Background:          "bg:#0d1117",
		chroma.Text:                "#c9d1d9",
		chroma.Comment:             "italic #8b949e",
		chroma.CommentPreproc:      "#ff7b72",
		chroma.Keyword:             "bold #ff7b72",
		chroma.KeywordType:         "#79c0ff",
		chroma.Operator:            "bold #ff7b72",
		chroma.Punctuation:         "#c9d1d9",
		chroma.Name:                "#c9d1d9",
		chroma.NameBuiltin:         "#79c0ff",
		chroma.NameTag:             "bold #7ee787",
		chroma.NameAttribute:       "#79c0ff",
		chroma.NameClass:           "bold #f0883e",
		chroma.NameConstant:        "#79c0ff",
		chroma.NameDecorator:       "bold #d2a8ff",
		chroma.NameFunction:        "#d2a8ff",
		chroma.LiteralNumber:       "#79c0ff",
		chroma.LiteralString:       "#a5d6ff",
		chroma.LiteralStringEscape: "bold #79c0ff",
		chroma.GenericEmph:         "italic",
		chroma.GenericStrong:       "bold",
		chroma.GenericSubheading:   "#8b949e",
	}
	if daltonized {
		entries[chroma.GenericInserted] = "#58a6ff" // blue
		entries[chroma.GenericDeleted] = "#f85149"  // keep red
		entries[chroma.NameTag] = "bold #58a6ff"    // blue instead of green
		entries[chroma.NameFunction] = "#58a6ff"    // blue
	} else {
		entries[chroma.GenericInserted] = "#3fb950" // green
		entries[chroma.GenericDeleted] = "#f85149"  // red
	}
	return entries
}

func githubLightDaltonizedChromaEntries() chroma.StyleEntries {
	return chroma.StyleEntries{
		chroma.Background:          "bg:#ffffff",
		chroma.Text:                "#24292e",
		chroma.Comment:             "italic #6a737d",
		chroma.CommentPreproc:      "#e36209",
		chroma.Keyword:             "bold #d73a49",
		chroma.KeywordType:         "#0000ff",
		chroma.Operator:            "bold #d73a49",
		chroma.Punctuation:         "#24292e",
		chroma.Name:                "#24292e",
		chroma.NameBuiltin:         "#005cc5",
		chroma.NameTag:             "bold #22863a",
		chroma.NameAttribute:       "#6f42c1",
		chroma.NameClass:           "bold #6f42c1",
		chroma.NameConstant:        "#005cc5",
		chroma.NameDecorator:       "bold #6f42c1",
		chroma.NameFunction:        "#005cc5", // blue instead of green
		chroma.LiteralNumber:       "#005cc5", // blue instead of green
		chroma.LiteralString:       "#032f62",
		chroma.LiteralStringEscape: "bold #032f62",
		chroma.GenericDeleted:      "#b31d28",
		chroma.GenericEmph:         "italic",
		chroma.GenericInserted:     "#005cc5", // blue instead of green
		chroma.GenericStrong:       "bold",
		chroma.GenericSubheading:   "bold #6a737d",
	}
}

// themeColors holds the resolved color values for a given theme.
type themeColors struct {
	addedFg   color.Color
	removedFg color.Color
	addedBg   color.Color
	removedBg color.Color

	cyan   color.Color
	yellow color.Color
	dim    color.Color
	white  color.Color
	border color.Color

	dimGreen  color.Color
	dimRed    color.Color
	dimYellow color.Color

	gutterAddedFg   color.Color
	gutterRemovedFg color.Color

	markBg color.Color
	markFg color.Color // foreground for the leading bookmark stripe
}

// paletteFor returns the color palette for the given theme.
// isDark is only consulted for ThemeAuto and ThemeAutoDaltonized.
func paletteFor(t Theme, isDark bool) themeColors {
	switch t {
	case ThemeCharmtoneDark:
		return charmtoneDarkPalette()
	case ThemeCharmtoneDarkDaltonized:
		return charmtoneDarkDaltonizedPalette()
	case ThemeCharmtoneLight:
		return charmtoneLightPalette()
	case ThemeCharmtoneLightDaltonized:
		return charmtoneLightDaltonizedPalette()
	case ThemeGithubDark:
		return githubDarkPalette()
	case ThemeGithubDarkDaltonized:
		return githubDarkDaltonizedPalette()
	case ThemeGithubLight:
		return githubLightPalette()
	case ThemeGithubLightDaltonized:
		return githubLightDaltonizedPalette()
	case ThemeAutoDaltonized:
		return autoDaltonizedPalette()
	default: // ThemeAuto
		return autoPalette()
	}
}

func autoPalette() themeColors {
	return themeColors{
		addedFg:         lipgloss.Green,
		removedFg:       lipgloss.Red,
		addedBg:         lipgloss.Color("#1a2e1f"),
		removedBg:       lipgloss.Color("#2e1a1c"),
		cyan:            lipgloss.Cyan,
		yellow:          lipgloss.Yellow,
		dim:             lipgloss.BrightBlack,
		white:           lipgloss.White,
		border:          lipgloss.BrightBlack,
		dimGreen:        lipgloss.Color("28"),
		dimRed:          lipgloss.Color("88"),
		dimYellow:       lipgloss.Yellow,
		gutterAddedFg:   lipgloss.Color("28"),
		gutterRemovedFg: lipgloss.Color("88"),
		markBg:          lipgloss.Color("#2d4a7a"),
		markFg:          lipgloss.Color("#5b9bd5"),
	}
}

func autoDaltonizedPalette() themeColors {
	p := autoPalette()
	p.addedFg = lipgloss.Blue
	p.addedBg = lipgloss.Color("#1a1f2e")
	p.gutterAddedFg = lipgloss.Color("27") // ANSI 256: muted blue
	p.markBg = lipgloss.Color("#5a2d6e")   // purple — distinct from blue addedBg
	p.markFg = lipgloss.Color("#c084fc")
	return p
}

func charmtoneDarkPalette() themeColors {
	return themeColors{
		addedFg:   lipgloss.Color("#00FFB2"), // Julep
		removedFg: lipgloss.Color("#FF388B"), // Cherry
		addedBg:   lipgloss.Color("#1C3634"), // Spinach
		removedBg: lipgloss.Color("#412130"), // Toast
		cyan:      lipgloss.Color("#00A4FF"), // Malibu
		yellow:    lipgloss.Color("#E8FF27"), // Citron
		dim:       lipgloss.Color("#605F6B"), // Oyster
		white:     lipgloss.Color("#DFDBDD"), // Ash
		border:    lipgloss.Color("#4D4C57"), // Iron
		dimGreen:  lipgloss.Color("#00A475"), // Pickle
		dimRed:    lipgloss.Color("#AB2454"), // Pom
		dimYellow: lipgloss.Color("#858392"), // Squid
		markBg:    lipgloss.Color("#2d4a7a"), // saturated blue, distinct from green/red diff bgs
		markFg:    lipgloss.Color("#4FBEFE"), // Sardine
	}
}

func charmtoneDarkDaltonizedPalette() themeColors {
	p := charmtoneDarkPalette()
	p.addedFg = lipgloss.Color("#4FBEFE")  // Sardine (blue)
	p.addedBg = lipgloss.Color("#0F2A4A")  // deep navy
	p.dimGreen = lipgloss.Color("#007AB8") // Damson
	p.markBg = lipgloss.Color("#5a2d6e")   // purple — distinct from blue addedBg
	p.markFg = lipgloss.Color("#c084fc")
	return p
}

func charmtoneLightPalette() themeColors {
	return themeColors{
		addedFg:   lipgloss.Color("#166534"),
		removedFg: lipgloss.Color("#9f1239"),
		addedBg:   lipgloss.Color("#d1fae5"),
		removedBg: lipgloss.Color("#ffd7d5"),
		cyan:      lipgloss.Color("#0369a1"),
		yellow:    lipgloss.Color("#92400e"),
		dim:       lipgloss.Color("#6b7280"),
		white:     lipgloss.Color("#111827"),
		border:    lipgloss.Color("#d1d5db"),
		dimGreen:  lipgloss.Color("#166534"),
		dimRed:    lipgloss.Color("#9f1239"),
		dimYellow: lipgloss.Color("#92400e"),
		markBg:    lipgloss.Color("#dbeafe"),
		markFg:    lipgloss.Color("#1d4ed8"),
	}
}

func charmtoneLightDaltonizedPalette() themeColors {
	p := charmtoneLightPalette()
	p.addedFg = lipgloss.Color("#1d4ed8")
	p.addedBg = lipgloss.Color("#dbeafe")
	p.dimGreen = lipgloss.Color("#1e40af")
	p.markBg = lipgloss.Color("#f3e8ff") // light purple — distinct from blue addedBg
	p.markFg = lipgloss.Color("#7c3aed")
	return p
}

func githubDarkPalette() themeColors {
	return themeColors{
		addedFg:   lipgloss.Color("#3fb950"),
		removedFg: lipgloss.Color("#f85149"),
		addedBg:   lipgloss.Color("#0d2818"),
		removedBg: lipgloss.Color("#3d0b0b"),
		cyan:      lipgloss.Color("#58a6ff"),
		yellow:    lipgloss.Color("#e3b341"),
		dim:       lipgloss.Color("#8b949e"),
		white:     lipgloss.Color("#c9d1d9"),
		border:    lipgloss.Color("#30363d"),
		dimGreen:  lipgloss.Color("#196c2e"),
		dimRed:    lipgloss.Color("#67060c"),
		dimYellow: lipgloss.Color("#9e6a03"),
		markBg:    lipgloss.Color("#1f4a8a"),
		markFg:    lipgloss.Color("#58a6ff"),
	}
}

func githubDarkDaltonizedPalette() themeColors {
	p := githubDarkPalette()
	p.addedFg = lipgloss.Color("#58a6ff") // GitHub blue
	p.addedBg = lipgloss.Color("#0d1f3d") // dark blue bg
	p.dimGreen = lipgloss.Color("#1f6feb")
	p.markBg = lipgloss.Color("#5a2d6e") // purple — distinct from blue addedBg
	p.markFg = lipgloss.Color("#c084fc")
	return p
}

func githubLightPalette() themeColors {
	return themeColors{
		addedFg:   lipgloss.Color("#166534"),
		removedFg: lipgloss.Color("#9f1239"),
		addedBg:   lipgloss.Color("#d1fae5"),
		removedBg: lipgloss.Color("#ffd7d5"),
		cyan:      lipgloss.Color("#0369a1"),
		yellow:    lipgloss.Color("#92400e"),
		dim:       lipgloss.Color("#6b7280"),
		white:     lipgloss.Color("#111827"),
		border:    lipgloss.Color("#d1d5db"),
		dimGreen:  lipgloss.Color("#166534"),
		dimRed:    lipgloss.Color("#9f1239"),
		dimYellow: lipgloss.Color("#92400e"),
		markBg:    lipgloss.Color("#dbeafe"),
		markFg:    lipgloss.Color("#1d4ed8"),
	}
}

func githubLightDaltonizedPalette() themeColors {
	p := githubLightPalette()
	p.addedFg = lipgloss.Color("#1d4ed8")
	p.addedBg = lipgloss.Color("#dbeafe")
	p.dimGreen = lipgloss.Color("#1e40af")
	p.markBg = lipgloss.Color("#f3e8ff") // light purple — distinct from blue addedBg
	p.markFg = lipgloss.Color("#7c3aed")
	return p
}

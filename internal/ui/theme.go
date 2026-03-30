package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
)

// Theme identifies the active color palette.
type Theme string

const (
	ThemeAuto            Theme = "auto"
	ThemeDark            Theme = "dark"
	ThemeLight           Theme = "light"
	ThemeDarkDaltonized  Theme = "dark-daltonized"
	ThemeLightDaltonized Theme = "light-daltonized"
)

// ValidThemes lists all accepted --theme values.
var ValidThemes = []Theme{ThemeAuto, ThemeDark, ThemeLight, ThemeDarkDaltonized, ThemeLightDaltonized}

// IsValidTheme reports whether t is a recognized theme name.
func IsValidTheme(t Theme) bool {
	for _, v := range ValidThemes {
		if t == v {
			return true
		}
	}
	return false
}

// chromaStyleFor returns the chroma style name for light themes.
// Dark themes use chromaStyleEntries() instead.
func chromaStyleFor(isDark bool) string {
	return "github"
}

// chromaStyleEntries returns a chroma.StyleEntries map using the Charmtone
// palette for dark themes. Returns nil for light themes (use named style).
// daltonized swaps green→blue for red-green colorblind users.
func chromaStyleEntries(isDark, daltonized bool) chroma.StyleEntries {
	if !isDark {
		return nil
	}

	entries := chroma.StyleEntries{
		chroma.Background:          "bg:#3A3943",
		chroma.Text:                "#BFBCC8",
		chroma.Error:               "bg:#EB4268 #FFFAF1",
		chroma.Comment:             "#605F6B", // Oyster
		chroma.CommentPreproc:      "#FF6E63", // Bengal
		chroma.Keyword:             "#00A4FF", // Malibu — blue, safe
		chroma.KeywordReserved:     "#FF4FBF", // Pony — pink, safe
		chroma.KeywordNamespace:    "#FF4FBF", // Pony
		chroma.KeywordType:         "#7272FF", // Guppy — blue, safe
		chroma.Punctuation:         "#E8FE96", // Zest — yellow, safe
		chroma.Name:                "#BFBCC8",
		chroma.NameBuiltin:         "#FF79D0", // Cheeky — pink, safe
		chroma.NameTag:             "#D46EFF", // Mauve — purple, safe
		chroma.NameAttribute:       "#8B75FF", // Hazy — purple, safe
		chroma.NameClass:           "bold underline #F1EFEF",
		chroma.NameConstant:        "#F1EFEF",
		chroma.NameDecorator:       "#E8FF27", // Citron — yellow, safe
		chroma.NameOther:           "#BFBCC8",
		chroma.Literal:             "#BF976F", // Cumin — brown/orange, safe
		chroma.LiteralString:       "#BF976F", // Cumin
		chroma.LiteralStringEscape: "#68FFD6", // Bok — teal, safe
		chroma.GenericEmph:         "italic",
		chroma.GenericStrong:       "bold",
		chroma.GenericSubheading:   "#858392", // Squid
	}

	if daltonized {
		entries[chroma.NameFunction] = "#4FBEFE"   // Sardine (blue) instead of Guac (green)
		entries[chroma.Operator] = "#E8FE96"        // Zest (yellow) instead of Salmon (pink-red)
		entries[chroma.GenericInserted] = "#4FBEFE" // Sardine (blue)
		entries[chroma.GenericDeleted] = "#FF985A"  // Tang (orange)
		entries[chroma.LiteralNumber] = "#4FBEFE"   // Sardine (blue)
	} else {
		entries[chroma.NameFunction] = "#12C78F"   // Guac (green)
		entries[chroma.Operator] = "#FF7F90"        // Salmon (pink)
		entries[chroma.GenericInserted] = "#12C78F" // Guac (green)
		entries[chroma.GenericDeleted] = "#FF577D"  // Coral (red)
		entries[chroma.LiteralNumber] = "#00FFB2"   // Julep (green)
	}

	return entries
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

	markBg color.Color
}

// paletteFor returns the color palette for the given theme and terminal background.
func paletteFor(t Theme, isDark bool) themeColors {
	switch t {
	case ThemeDarkDaltonized:
		return darkDaltonizedPalette()
	case ThemeLightDaltonized:
		return lightDaltonizedPalette()
	case ThemeLight:
		return lightPalette()
	case ThemeDark:
		return darkPalette()
	default: // ThemeAuto
		if isDark {
			return darkPalette()
		}
		return lightPalette()
	}
}

func darkPalette() themeColors {
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
		markBg:    lipgloss.Color("#18463D"), // Gator
	}
}

func lightPalette() themeColors {
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
	}
}

func darkDaltonizedPalette() themeColors {
	return themeColors{
		addedFg:   lipgloss.Color("#4FBEFE"), // Sardine (blue)
		removedFg: lipgloss.Color("#FF388B"), // Cherry
		addedBg:   lipgloss.Color("#0F2A4A"), // deep navy
		removedBg: lipgloss.Color("#412130"), // Toast
		cyan:      lipgloss.Color("#00A4FF"), // Malibu
		yellow:    lipgloss.Color("#E8FF27"), // Citron
		dim:       lipgloss.Color("#605F6B"), // Oyster
		white:     lipgloss.Color("#DFDBDD"), // Ash
		border:    lipgloss.Color("#4D4C57"), // Iron
		dimGreen:  lipgloss.Color("#007AB8"), // Damson
		dimRed:    lipgloss.Color("#AB2454"), // Pom
		dimYellow: lipgloss.Color("#858392"), // Squid
		markBg:    lipgloss.Color("#0F2A4A"), // navy
	}
}

func lightDaltonizedPalette() themeColors {
	return themeColors{
		addedFg:   lipgloss.Color("#1d4ed8"),
		removedFg: lipgloss.Color("#9f1239"),
		addedBg:   lipgloss.Color("#dbeafe"),
		removedBg: lipgloss.Color("#ffd7d5"),
		cyan:      lipgloss.Color("#0369a1"),
		yellow:    lipgloss.Color("#92400e"),
		dim:       lipgloss.Color("#6b7280"),
		white:     lipgloss.Color("#111827"),
		border:    lipgloss.Color("#d1d5db"),
		dimGreen:  lipgloss.Color("#1e40af"),
		dimRed:    lipgloss.Color("#9f1239"),
		dimYellow: lipgloss.Color("#92400e"),
		markBg:    lipgloss.Color("#dbeafe"),
	}
}

package ui

import "github.com/charmbracelet/lipgloss"

// Theme identifies the active color palette.
type Theme string

const (
	ThemeDark            Theme = "dark"
	ThemeLight           Theme = "light"
	ThemeDarkDaltonized  Theme = "dark-daltonized"
	ThemeLightDaltonized Theme = "light-daltonized"
)

// ValidThemes lists all accepted --theme values.
var ValidThemes = []Theme{ThemeDark, ThemeLight, ThemeDarkDaltonized, ThemeLightDaltonized}

// IsValidTheme reports whether t is a recognized theme name.
func IsValidTheme(t Theme) bool {
	for _, v := range ValidThemes {
		if t == v {
			return true
		}
	}
	return false
}

// themeColors holds the resolved color values for a given theme.
type themeColors struct {
	addedFg   lipgloss.Color
	removedFg lipgloss.Color
	addedBg   lipgloss.Color
	removedBg lipgloss.Color

	cyan   lipgloss.Color
	yellow lipgloss.Color
	dim    lipgloss.Color
	white  lipgloss.Color
	border lipgloss.Color

	dimGreen  lipgloss.Color
	dimRed    lipgloss.Color
	dimYellow lipgloss.Color
}

func paletteFor(t Theme) themeColors {
	switch t {
	case ThemeLight:
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
		}
	case ThemeDarkDaltonized:
		return themeColors{
			addedFg:   lipgloss.Color("#60a5fa"),
			removedFg: lipgloss.Color("#ef4444"),
			addedBg:   lipgloss.Color("#0f2a4a"),
			removedBg: lipgloss.Color("#6d0f0f"),
			cyan:      lipgloss.Color("#06b6d4"),
			yellow:    lipgloss.Color("#eab308"),
			dim:       lipgloss.Color("#6b7280"),
			white:     lipgloss.Color("#e5e7eb"),
			border:    lipgloss.Color("#374151"),
			dimGreen:  lipgloss.Color("#1e3a5f"),
			dimRed:    lipgloss.Color("#7f1d1d"),
			dimYellow: lipgloss.Color("#854d0e"),
		}
	case ThemeLightDaltonized:
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
		}
	default: // ThemeDark
		return themeColors{
			addedFg:   lipgloss.Color("#22c55e"),
			removedFg: lipgloss.Color("#ef4444"),
			addedBg:   lipgloss.Color("#1a3d1a"),
			removedBg: lipgloss.Color("#6d0f0f"),
			cyan:      lipgloss.Color("#06b6d4"),
			yellow:    lipgloss.Color("#eab308"),
			dim:       lipgloss.Color("#6b7280"),
			white:     lipgloss.Color("#e5e7eb"),
			border:    lipgloss.Color("#374151"),
			dimGreen:  lipgloss.Color("#166534"),
			dimRed:    lipgloss.Color("#7f1d1d"),
			dimYellow: lipgloss.Color("#854d0e"),
		}
	}
}

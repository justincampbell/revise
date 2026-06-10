package ui

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
)

// luminance returns the perceived relative luminance of c on a 0 (black) to 1
// (white) scale.
func luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA() // each channel is 0..65535
	return (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 65535.0
}

// TestAutoPalette_AdaptsToTerminalBackground guards against #198: on a light
// terminal the auto theme must use light diff-line backgrounds, otherwise dark
// syntax-highlight foregrounds are baked over a dark background and become
// unreadable ("code bg is always dark").
func TestAutoPalette_AdaptsToTerminalBackground(t *testing.T) {
	dark := paletteFor(ThemeAuto, true)
	light := paletteFor(ThemeAuto, false)

	// Dark terminal: diff backgrounds are dark.
	assert.Less(t, luminance(dark.addedBg), 0.5, "auto dark addedBg should be dark")
	assert.Less(t, luminance(dark.removedBg), 0.5, "auto dark removedBg should be dark")

	// Light terminal: diff backgrounds must be light.
	assert.Greater(t, luminance(light.addedBg), 0.5, "auto light addedBg should be light")
	assert.Greater(t, luminance(light.removedBg), 0.5, "auto light removedBg should be light")

	// The prominent foreground must be dark on a light terminal so file names,
	// the selected row, and help text stay readable.
	assert.Less(t, luminance(light.white), 0.5, "auto light prominent fg should be dark")
}

func TestAutoDaltonizedPalette_AdaptsToTerminalBackground(t *testing.T) {
	dark := paletteFor(ThemeAutoDaltonized, true)
	light := paletteFor(ThemeAutoDaltonized, false)

	assert.Less(t, luminance(dark.addedBg), 0.5, "auto-daltonized dark addedBg should be dark")
	assert.Greater(t, luminance(light.addedBg), 0.5, "auto-daltonized light addedBg should be light")
	assert.Greater(t, luminance(light.removedBg), 0.5, "auto-daltonized light removedBg should be light")
	assert.Less(t, luminance(light.white), 0.5, "auto-daltonized light prominent fg should be dark")
}

// TestHighlightLine_AutoLightIsReadable verifies the auto theme bakes a light
// background behind syntax highlighting on a light terminal (#198).
func TestHighlightLine_AutoLightIsReadable(t *testing.T) {
	orig := noColor
	noColor = false
	origTheme := activeTheme
	origIsDark := activeIsDark
	defer func() {
		noColor = orig
		SetTheme(origTheme, origIsDark)
	}()

	SetTheme(ThemeAuto, false)
	bg := paletteFor(ThemeAuto, false).addedBg
	out, ok := highlightLine("package main", "main.go", bg, 0, "")
	assert.True(t, ok)

	// The dark added background from the dark palette must not appear.
	darkBg := paletteFor(ThemeAuto, true).addedBg
	dr, dg, db, _ := darkBg.RGBA()
	assert.NotContains(t, out, sgrBg(dr, dg, db), "auto light must not bake the dark added background")
}

// sgrBg builds the truecolor background SGR parameter fragment lipgloss emits
// for an RGB color (channels are 0..65535 from color.Color, shifted to 0..255).
func sgrBg(r, g, b uint32) string {
	return fmt.Sprintf("48;2;%d;%d;%d", r>>8, g>>8, b>>8)
}

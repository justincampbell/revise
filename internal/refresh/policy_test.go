package refresh

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPolicy_NextDelay(t *testing.T) {
	p := Policy{Min: 2 * time.Second, Max: 30 * time.Second, Multiplier: 5}

	cases := []struct {
		name         string
		lastDuration time.Duration
		want         time.Duration
	}{
		{"zero duration clamps to Min", 0, 2 * time.Second},
		{"very fast clamps to Min", 100 * time.Millisecond, 2 * time.Second},
		{"in-band scales by multiplier", 1 * time.Second, 5 * time.Second},
		{"larger in-band", 3 * time.Second, 15 * time.Second},
		{"at boundary stays in-band", 6 * time.Second, 30 * time.Second},
		{"slow clamps to Max", 10 * time.Second, 30 * time.Second},
		{"very slow clamps to Max", 1 * time.Minute, 30 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, p.NextDelay(tc.lastDuration))
		})
	}
}

func TestPolicy_Debounce(t *testing.T) {
	p := Policy{Min: 2 * time.Second, Max: 30 * time.Second, Multiplier: 5}
	now := time.Now()

	cases := []struct {
		name      string
		lastStart time.Time
		want      time.Duration
	}{
		{"zero lastStart fires immediately", time.Time{}, 0},
		{"long ago fires immediately", now.Add(-10 * time.Second), 0},
		{"exactly Min ago fires immediately", now.Add(-2 * time.Second), 0},
		{"recent waits remainder", now.Add(-500 * time.Millisecond), 1500 * time.Millisecond},
		{"just now waits ~Min", now.Add(-1 * time.Millisecond), 1999 * time.Millisecond},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := p.Debounce(tc.lastStart, now)
			// Tolerate small drift on the "just now" math
			assert.InDelta(t, tc.want.Nanoseconds(), got.Nanoseconds(), float64(2*time.Millisecond.Nanoseconds()))
		})
	}
}

func TestPolicy_Default(t *testing.T) {
	assert.Equal(t, 2*time.Second, Default.Min)
	assert.Equal(t, 30*time.Second, Default.Max)
	assert.Equal(t, 5, Default.Multiplier)
}

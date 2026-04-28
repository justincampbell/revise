// Package refresh provides cadence policy for revise's auto-refresh loop.
//
// The Policy type decides two things: how long to wait until the next
// scheduled poll (based on the duration of the last one — adaptive
// back-off when git ops are slow), and whether an externally-requested
// refresh (focus, fsnotify event, etc.) can fire now or must wait for
// debounce. The logic is pure — no time.Now in NextDelay, no goroutines,
// no UI dependencies — so it's trivially testable in isolation.
package refresh

import "time"

// Policy controls auto-refresh cadence.
type Policy struct {
	// Min is the floor for any inter-poll delay. Refreshes can never
	// fire faster than once per Min; this is the debounce window.
	Min time.Duration

	// Max is the ceiling for any inter-poll delay. Even when the last
	// poll was very slow, the next one fires no later than Max from now.
	Max time.Duration

	// Multiplier sets the back-off curve: NextDelay is the last poll's
	// duration times Multiplier, clamped to [Min, Max]. A value of 5
	// keeps the steady-state poll cost at roughly 1/5 of total time.
	Multiplier int
}

// Default is the policy used when no override is provided.
var Default = Policy{
	Min:        2 * time.Second,
	Max:        30 * time.Second,
	Multiplier: 5,
}

// NextDelay returns how long to wait before the next scheduled poll,
// given how long the previous poll took. Scales with lastDuration so
// that under contention (10-20 instances slowing each other down) the
// cadence self-throttles instead of stacking I/O.
func (p Policy) NextDelay(lastDuration time.Duration) time.Duration {
	next := lastDuration * time.Duration(p.Multiplier)
	if next < p.Min {
		next = p.Min
	}
	if next > p.Max {
		next = p.Max
	}
	return next
}

// Debounce returns how long an externally-requested refresh must wait
// before it can fire, given when the last poll started. Returns 0 if
// it can fire immediately. Used by the FocusMsg / fsEvent paths so a
// burst of triggers collapses into one poll.
func (p Policy) Debounce(lastStart, now time.Time) time.Duration {
	if lastStart.IsZero() {
		return 0
	}
	elapsed := now.Sub(lastStart)
	if elapsed >= p.Min {
		return 0
	}
	return p.Min - elapsed
}

package joblearn

import "time"

// Clock returns the current time. Implementations accept a Clock so recorded
// timestamps are deterministic in tests.
type Clock interface {
	Now() time.Time
}

// FixedClock is a deterministic clock that advances by a fixed step per call.
type FixedClock struct {
	start time.Time
	step  time.Duration
	calls int
}

// NewFixedClock returns a clock starting at the Unix epoch, one second per call.
func NewFixedClock() *FixedClock {
	return &FixedClock{start: time.Unix(0, 0).UTC(), step: time.Second}
}

// Now implements Clock.
func (c *FixedClock) Now() time.Time {
	t := c.start.Add(time.Duration(c.calls) * c.step)
	c.calls++
	return t
}

// SystemClock is a Clock backed by wall time in UTC.
type SystemClock struct{}

// Now implements Clock.
func (SystemClock) Now() time.Time { return time.Now().UTC() }

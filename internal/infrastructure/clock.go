package infrastructure

import "github.com/ccheney/bd-claim/internal/domain"

// SystemClock implements ClockPort using system time.
type SystemClock struct{}

// NewSystemClock creates a new SystemClock.
func NewSystemClock() *SystemClock {
	return &SystemClock{}
}

// Now returns the current time.
func (c *SystemClock) Now() domain.Timestamp {
	return domain.Now()
}

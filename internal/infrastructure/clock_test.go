package infrastructure

import (
	"testing"
	"time"
)

func TestSystemClock_Now(t *testing.T) {
	clock := NewSystemClock()

	before := time.Now()
	ts := clock.Now()
	after := time.Now()

	tsTime := ts.Time()
	if tsTime.Before(before) || tsTime.After(after) {
		t.Error("timestamp should be between before and after")
	}
}

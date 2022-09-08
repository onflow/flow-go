package unittest

import (
	"sync"
	"time"
)

// TestTime is a fake time used for testing.
type TestTime struct {
	mu  sync.Mutex
	cur time.Time // current fake time
}

// Now returns the current fake time.
func (tt *TestTime) Now() time.Time {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	return tt.cur
}

// Since returns the fake time since the given time.
func (tt *TestTime) Since(t time.Time) time.Duration {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	return tt.cur.Sub(t)
}

// Advance advances the fake time.
func (tt *TestTime) Advance(dur time.Duration) {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	tt.cur = tt.cur.Add(dur)
}

// NewTestTime returns new TestTime.
func NewTestTime() *TestTime {
	return &TestTime{
		cur: time.Now(),
	}
}

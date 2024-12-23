package concurrentticker

import (
	"sync"
	"time"
)

// Ticker is a thread-safe ticker.
type Ticker struct {
	mu     sync.RWMutex
	ticker *time.Ticker
}

// NewTicker returns a new Ticker containing a channel that will send
// the current time on the channel after each tick.
func NewTicker(d time.Duration) *Ticker {
	return &Ticker{
		ticker: time.NewTicker(d),
	}
}

// Stop turns off a ticker. After Stop, no more ticks will be sent.
// Stop does not close the channel, to prevent a concurrent goroutine
// reading from the channel from seeing an erroneous "tick".
func (t *Ticker) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ticker.Stop()
}

// Reset stops a ticker and resets its period to the specified duration.
// The next tick will arrive after the new period elapses. The duration d
// must be greater than zero; if not, Reset will panic.
func (t *Ticker) Reset(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ticker.Reset(d)
}

func (t *Ticker) C() <-chan time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.ticker.C
}

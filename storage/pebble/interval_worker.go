package pebble

import (
	"context"
	"time"
)

// IntervalWorker runs a periodic task with support for context cancellation.
type IntervalWorker struct {
	interval time.Duration
}

// NewIntervalWorker initializes a new IntervalWorker.
func NewIntervalWorker(interval time.Duration) *IntervalWorker {
	return &IntervalWorker{
		interval: interval,
	}
}

// Run starts the worker and calls the provided function periodically.
// It stops and returns if the context is canceled.
func (iw *IntervalWorker) Run(ctx context.Context, f func()) {
	timer := time.NewTicker(iw.interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			// Stop if the context is canceled
			return
		case <-timer.C:
			f()
		}
	}
}

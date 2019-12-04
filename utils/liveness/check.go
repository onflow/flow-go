package liveness

import (
	"sync"
	"time"
)

// Check is a heartbeat style liveness reporter.
//
// It is not guaranteed to be safe to CheckIn across multiple goroutines.
// IsLive must be safe to be called concurrently with CheckIn.
type Check interface {
	CheckIn()
	IsLive(time.Duration) bool
}

// internalCheck implements a Check.
type internalCheck struct {
	lock             sync.RWMutex
	lastCheckIn      time.Time
	defaultTolerance time.Duration
}

// CheckIn adds a heartbeat at the current time.
func (c *internalCheck) CheckIn() {
	c.lock.Lock()
	c.lastCheckIn = time.Now()
	c.lock.Unlock()
}

// IsLive checks if we are still live against the given the tolerace between hearbeats.
//
// If tolerance is 0, the default tolerance is used.
func (c *internalCheck) IsLive(tolerance time.Duration) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if tolerance == 0 {
		tolerance = c.defaultTolerance
	}

	return c.lastCheckIn.Add(tolerance).After(time.Now())
}

// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package engine

import (
	"sync"
)

// Unit handles synchronization management, startup, and shutdown for engines.
type Unit struct {
	wg         sync.WaitGroup // tracks in-progress functions
	once       sync.Once      // ensures that the done channel is only closed once
	quit       chan struct{}  // used to signal that shutdown has started
	sync.Mutex                // can be used to synchronize the engine
}

// NewUnit returns a new unit.
func NewUnit() *Unit {
	return &Unit{
		quit: make(chan struct{}),
	}
}

// Do synchronously executes the input function f unless the unit has shut down.
// It returns the result of f. If f is executed, the unit will not shut down
// until after f returns.
func (u *Unit) Do(f func() error) error {
	select {
	case <-u.quit:
		return nil
	default:
	}
	u.wg.Add(1)
	defer u.wg.Done()
	return f()
}

// Launch asynchronously executes the input function unless the unit has shut
// down. If f is executed, the unit will not shut down until after f returns.
func (u *Unit) Launch(f func()) {
	select {
	case <-u.quit:
		return
	default:
	}
	u.wg.Add(1)
	go func() {
		defer u.wg.Done()
		f()
	}()
}

// Ready returns a channel that is closed when the unit is ready. A unit is
// ready when the series of "check" functions are executed.
//
// The engine using the unit is responsible for defining these check functions
// as required.
func (u *Unit) Ready(checks ...func()) <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		for _, check := range checks {
			check()
		}
		close(ready)
	}()
	return ready
}

// Quit returns a channel that is closed when the unit begins to shut down.
func (u *Unit) Quit() <-chan struct{} {
	return u.quit
}

// Done returns a channel that is closed when the unit is done. A unit is done
// when (i) the series of "action" functions are executed and (ii) all pending
// functions invoked with `Do` or `Launch` have completed.
//
// The engine using the unit is responsible for defining these action functions
// as required.
func (u *Unit) Done(actions ...func()) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		u.once.Do(func() {
			close(u.quit)
		})
		for _, action := range actions {
			action()
		}
		u.wg.Wait()
		close(done)
	}()
	return done
}

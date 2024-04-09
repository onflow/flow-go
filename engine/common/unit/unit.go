package unit

import (
	"context"
	"sync"
	"time"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// Unit handles synchronization management
type Unit interface {
	component.Component
	ShutdownSignal() <-chan struct{}

	Do(f func() error) error
	Launch(f func(context.Context))
	LaunchAfter(delay time.Duration, f func(context.Context))
	LaunchPeriodically(f func(context.Context), interval time.Duration, delay time.Duration)

	AddReadyCallbacks(checks ...func())
	AddDoneCallbacks(actions ...func())
}

var _ Unit = (*unitImp)(nil)

type unitImp struct {
	*component.ComponentManager
	sync.Mutex // can be used to synchronize the engine

	wg   sync.WaitGroup             // tracks in-progress functions
	work chan func(context.Context) // used to pass work from Launch methods

	stopped   chan struct{} // used to signal the unit to stop admitting work
	admitLock sync.Mutex    // used for synchronizing work admittance with shutdown

	preReadyFn func()
	preDoneFn  func()
}

func NewUnit() Unit {
	u := &unitImp{
		work:    make(chan func(context.Context)),
		stopped: make(chan struct{}),
	}

	u.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(u.workerFactory).
		AddWorker(u.lifecycle).
		Build()

	return u
}

// admit returns true if the unit is still admitting work, and false otherwise.
//
// This is used to prevent race conditions when adding new work between the initial check if unit is
// still accepting new work, and when Wait is called on the waitgroup. The API guarantees that once
// a callback has started, it will finish before the unit is stopped.
func (u *unitImp) admit() bool {
	u.admitLock.Lock()
	defer u.admitLock.Unlock()

	select {
	case <-u.stopped:
		return false
	default:
	}

	u.wg.Add(1)
	return true
}

// stopAdmitting stops the unit from admitting new work.
func (u *unitImp) stopAdmitting() {
	u.admitLock.Lock()
	defer u.admitLock.Unlock()

	close(u.stopped)
}

func (u *unitImp) workerFactory(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	for {
		select {
		case <-u.stopped:
			return
		case f := <-u.work:
			if !u.admit() {
				return
			}

			go func() {
				defer u.wg.Done()
				f(ctx)
			}()
		}
	}
}

func (u *unitImp) lifecycle(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	if u.preReadyFn != nil {
		u.preReadyFn()
	}

	ready()
	<-ctx.Done()
	u.stopAdmitting()

	if u.preDoneFn != nil {
		u.preDoneFn()
	}

	u.wg.Wait()
}

// ComponentWorker is helper function to start the unit as a worker within a parent ComponentManager
func (u *unitImp) ComponentWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	u.Start(ctx)

	select {
	case <-ctx.Done():
	case <-u.Ready():
		ready()
	}

	<-u.Done()
}

// Do synchronously executes the input function f unless the unit has shut down.
// It returns the result of f. If f is executed, the unit will not shut down
// until after f returns.
func (u *unitImp) Do(f func() error) error {
	if !u.admit() {
		return nil
	}
	defer u.wg.Done()

	return f()
}

// Launch asynchronously executes the input function unless the unit has shut
// down. If f is executed, the unit will not shut down until after f returns.
func (u *unitImp) Launch(f func(ctx context.Context)) {
	select {
	// don't admit the work here, to avoid deadlock if the unit is shutting down.
	case <-u.stopped:
		return
	case u.work <- f:
	}
}

// LaunchAfter asynchronously executes the input function after a certain delay
// unless the unit has shut down.
func (u *unitImp) LaunchAfter(delay time.Duration, f func(context.Context)) {
	u.Launch(func(ctx context.Context) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
			f(ctx)
		}
	})
}

// LaunchPeriodically asynchronously executes the input function on `interval` periods
// unless the unit has shut down.
// If f is executed, the unit will not shut down until after f returns.
func (u *unitImp) LaunchPeriodically(f func(context.Context), interval time.Duration, delay time.Duration) {
	u.Launch(func(ctx context.Context) {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				f(ctx)
			}
		}
	})
}

// AddReadyCallbacks adds checks to be executed before the unit is ready.
// A unit is ready when the series of "check" functions are executed.
//
// The engine using the unit is responsible for defining these check functions
// as required.
func (u *unitImp) AddReadyCallbacks(checks ...func()) {
	u.preReadyFn = func() {
		for _, check := range checks {
			check()
		}
	}
}

// AddDoneCallbacks adds actions to be executed after the unit has shut down.
// A unit is done when
// (i) the series of "action" functions are executed and
// (ii) all pending functions invoked with `Do` or `Launch` have completed.
//
// The engine using the unit is responsible for defining these action functions
// as required.
func (u *unitImp) AddDoneCallbacks(actions ...func()) {
	u.preDoneFn = func() {
		for _, action := range actions {
			action()
		}
	}
}

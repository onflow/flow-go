package unit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

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

type unitImp struct {
	*component.ComponentManager

	wg   sync.WaitGroup
	work chan func(context.Context)

	preReadyFn func()
	postDoneFn func()
}

func NewUnit() Unit {
	u := &unitImp{
		work: make(chan func(context.Context)),
	}

	u.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(u.workerFactory).
		AddWorker(u.lifecycle).
		Build()

	return u
}

func (u *unitImp) workerFactory(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	for {
		select {
		case <-ctx.Done():
			return
		case f := <-u.work:
			u.wg.Add(1)
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

	if u.postDoneFn != nil {
		u.postDoneFn()
	}

	u.wg.Wait()
}

func (u *unitImp) Do(f func() error) error {
	select {
	case <-u.ShutdownSignal():
		return fmt.Errorf("unit is shutting down")
	default:
	}

	u.wg.Add(1)
	defer u.wg.Done()

	return f()
}

func (u *unitImp) Launch(f func(ctx context.Context)) {
	select {
	case <-u.ShutdownSignal():
		return
	case u.work <- f:
	}
}

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
func (u *unitImp) AddReadyCallbacks(checks ...func()) {
	u.preReadyFn = func() {
		for _, check := range checks {
			check()
		}
	}
}

// AddDoneCallbacks adds actions to be executed after the unit has shut down.
func (u *unitImp) AddDoneCallbacks(actions ...func()) {
	u.postDoneFn = func() {
		for _, action := range actions {
			action()
		}
	}
}

package module

import (
	"context"

	"github.com/onflow/flow-go/module/irrecoverable"
)

// ReadyDoneAware provides an easy interface to wait for module startup and shutdown.
// Modules that implement this interface only support a single start-stop cycle, and
// will not restart if Ready() is called again after shutdown has already commenced.
type ReadyDoneAware interface {
	// Ready commences startup of the module, and returns a ready channel that is closed once
	// startup has completed.
	// If shutdown has already commenced before this method is called for the first time,
	// startup will not be performed and the returned channel will never close.
	// This should be an idempotent method.
	Ready() <-chan struct{}

	// Done commences shutdown of the module, and returns a done channel that is closed once
	// shutdown has completed.
	// This should be an idempotent method.
	Done() <-chan struct{}
}

type NoopReadDoneAware struct{}

func (n *NoopReadDoneAware) Ready() <-chan struct{} {
	ready := make(chan struct{})
	defer close(ready)
	return ready
}

func (n *NoopReadDoneAware) Done() <-chan struct{} {
	done := make(chan struct{})
	defer close(done)
	return done
}

// Startable provides an interface to start a component. Once started, the component
// can be stopped by cancelling the given context.
type Startable interface {
	Start(irrecoverable.SignalerContext) error
}

type Component interface {
	Startable
	ReadyDoneAware
}

type ComponentFactory func() (Component, error)

// OnError reacts to an irrecoverable error
// It is meant to inspect the error, determining its type and seeing if e.g. a restart or some other measure is suitable,
// and optionally trigger the continuation provided by the caller (RunComponent), which defines what "a restart" means.
// Instead of restarting the component, it could also:
// - panic (in canary / benchmark)
// - log in various Error channels and / or send telemetry ...
type OnError = func(err error, triggerRestart func())

func RunComponent(ctx context.Context, componentFactory ComponentFactory, handler OnError) error {
	// reference to per-run signals for the component
	var component Component
	var cancel context.CancelFunc
	var done <-chan struct{}
	var irrecoverables chan error

	start := func() error {
		var err error // startup error, should be handled out of band

		component, err = componentFactory()
		if err != nil {
			return err // failure to generate the component, should be handles out-of-band because a restart won't help
		}

		// context used to run the component
		var runCtx context.Context
		runCtx, cancel = context.WithCancel(ctx)

		// signaler used for irrecoverables
		var signalingCtx irrecoverable.SignalerContext
		irrecoverables = make(chan error)
		signalingCtx = irrecoverable.WithSignaler(runCtx, irrecoverable.NewSignaler(irrecoverables))

		if err = component.Start(signalingCtx); err != nil {
			// failed to start component: this should not trigger a restart
			return err
		}

		// wait for Ready
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-component.Ready():
		}

		done = component.Done()
		return nil
	}

	for {
		if err := start(); err != nil {
			return err // failure to start
		}

		select {
		case err := <-irrecoverables:
			// shutdown the component
			cancel()

			// wait until it's done
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-done:
			}

			// send error to the handler programmed with a restart continuation
			restartChan := make(chan struct{})
			go handler(err, func() {
				close(restartChan)
			})

			// wait for handler to trigger restart or abort
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-restartChan:
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

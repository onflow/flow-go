package irrecoverable

import (
	"context"
	"log"
	"runtime"

	"github.com/onflow/flow-go/module"
)

// IrrecoverableSignaler sends the error out
type IrrecoverableSignaler struct {
	irrecoverableErrors chan<- error
}

// ThrowIrrecoverable is a narrow drop-in replacement for panic, log.Fatal, log.Panic, etc
// anywhere there's something connected to the irrecoverable Error channel
func (e *IrrecoverableSignaler) ThrowIrrecoverable(err error) {
	e.irrecoverableErrors <- err
	runtime.Goexit()
}

// OnIrrecoverable reacts to the error
// It could:
// - restart the component (in production) after cleanup
// - panic (in canary / benchmark)
// - log in various Error channels and / or send telemetry ...
// does not do the plumbing of connecting to the channel
type OnIrrecoverable = func(err error, andThen func() error)

///////////////////////////////////////////////////////
// Integrating the sending part it in a context      //
// for more on contexts: https://go.dev/blog/context //
///////////////////////////////////////////////////////

// We define a constrained interface to provide a drop-in replacement for context.Context
// including in interfaces that compose it.
type IrrecoverableSignalerContext interface {
	context.Context
	ThrowIrrecoverable(err error) // delegates to the signaler
	sealed()                      // private, to constrain builder to using WithIrrecoverableSignal
}

// private, to force context derivation / WithIrrecoverableSignal
type irrecoverableSignalerCtxt struct {
	context.Context
	signaler *IrrecoverableSignaler
}

func (irs irrecoverableSignalerCtxt) sealed() {}

// Drop-in replacement for panic, log.Fatal, log.Panic, etc
// to use when we are able to get an IrrecoverableSignalerContext and thread it down in the component
func (irs irrecoverableSignalerCtxt) ThrowIrrecoverable(err error) {
	irs.signaler.ThrowIrrecoverable(err)
}

// the One True Way of getting an IrrecoverableSignalerContext
func WithIrrecoverableSignal(ctx context.Context, sig *IrrecoverableSignaler) IrrecoverableSignalerContext {
	return irrecoverableSignalerCtxt{ctx, sig}
}

// If we have an IrrecoverableSignalerContext, we can directly ctx.ThrowIrrecoverable.
//
// But a lot of library methods expect context.Context, & we want to pass the same w/o boilerplate
// Moreover, we could have built with: context.WithCancel(context.WithIrrecoverableSignal(ctx, sig), ...)
// "downcasting" to context.Context. Yet, we can still type-assert and recover.
//
// ThrowIrrecoverable can be a drop-in replacement anywhere we have a context.Context likely
// to support Irrecoverables. Note: this is not a method
func ThrowIrrecoverable(ctx context.Context, err error) {
	signalerAbleContext, ok := ctx.(IrrecoverableSignalerContext)
	if ok {
		signalerAbleContext.ThrowIrrecoverable(err)
	}
	// Be spectacular on how this does not -but should- handle irrecoverables:
	log.Fatalf("Irrecoverable error signaler not found for context, please implement! Unhandled irrecoverable error %v", err)
}

////////////////////////////////////////////////
// Integrating it w/ ReadyDoneAware & friends //
////////////////////////////////////////////////

// If we want to do it using interface composition (see module.Component), we will need to build on the existing interfaces
// note the irrecoverable management needs to be:
// - set up before the call to start (if the start itself meets an irrecoverable condition)
// - not throw / return error itself, except to an enclosing context

type Startable interface {
	Start(runCtx IrrecoverableSignalerContext) error
}

// later, in RunComponent, the plumbing happens ...
type Component interface {
	Startable
	module.ReadyDoneAware
}
type ComponentFactory func() (Component, error)

func RunComponent(parentCtx IrrecoverableSignalerContext, componentFactory ComponentFactory, handler OnIrrecoverable) error {

	// reference to per-run signals for the component
	var component Component
	var cancel context.CancelFunc
	var done <-chan struct{}
	var irrecoverables chan error

	// Tells us:
	// - how to get started
	// - how to create a closure to program the handler with a continuation
	restart := func() error {
		var err error // startup error, should be handled out of band

		component, err = componentFactory()
		if err != nil {
			return err // failure to generate the component, should be handles out-of-band because a restart won't help
		}

		// context used to run the component
		var runCtx context.Context
		runCtx, cancel = context.WithCancel(parentCtx)

		// signaler used for irrecoverables
		var signalingCtx IrrecoverableSignalerContext
		irrecoverables = make(chan error)
		signalingCtx = WithIrrecoverableSignal(runCtx, &IrrecoverableSignaler{irrecoverables})

		if err = component.Start(signalingCtx); err != nil {
			// failed to start component: this should not trigger a restart
			return err
		}
		// Anywhere inside the component, we can use signalingCtx.ThrowIrrecoverable(err), if the types support it
		// and ThrowIrrecoverable(signalingCtx, err) if not

		// wait for Ready
		<-component.Ready()

		done = component.Done()
		return nil
	}

	err := restart()
	if err != nil {
		return err // failure to start
	}

	for {
		select {
		case err := <-irrecoverables:
			// shutdown the component,
			cancel()
			// wait until it's done
			select {
			case <-parentCtx.Done():
				break
			case <-done:
			}
			// send error to the handler programmed with a restart continuation
			handler(err, restart)
		case <-done:
			// successful finish
			break
		case <-parentCtx.Done():
			// fail from above, shutdown and do not restart
			// TODO: maybe log parentCtx.Err()
			cancel()
			break
		}
	}

}

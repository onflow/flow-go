package module

import (
	"context"
	"sync"

	"github.com/onflow/flow-go/module/irrecoverable"
)

// WARNING: The semantics of this interface will be changing in the near future, with
// startup / shutdown capabilities being delegated to the Startable interface instead.
// For more details, see [FLIP 1167](https://github.com/onflow/flow-go/pull/1167)
//
// ReadyDoneAware provides an easy interface to wait for module startup and shutdown.
// Modules that implement this interface only support a single start-stop cycle, and
// will not restart if Ready() is called again after shutdown has already commenced.
type ReadyDoneAware interface {
	// Ready commences startup of the module, and returns a ready channel that is closed once
	// startup has completed. Note that the ready channel may never close if errors are
	// encountered during startup.
	// If shutdown has already commenced before this method is called for the first time,
	// startup will not be performed and the returned channel will also never close.
	// This should be an idempotent method.
	Ready() <-chan struct{}

	// Done commences shutdown of the module, and returns a done channel that is closed once
	// shutdown has completed. Note that the done channel should be closed even if errors are
	// encountered during shutdown.
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
	Start(irrecoverable.SignalerContext)
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

	start := func() (err error) {
		component, err = componentFactory()
		if err != nil {
			return // failure to generate the component, should be handled out-of-band because a restart won't help
		}

		// context used to run the component
		var runCtx context.Context
		runCtx, cancel = context.WithCancel(ctx)

		// signaler used for irrecoverables
		var signalingCtx irrecoverable.SignalerContext
		irrecoverables = make(chan error)
		signalingCtx = irrecoverable.WithSignaler(runCtx, irrecoverable.NewSignaler(irrecoverables))

		// the component must be started in a separate goroutine in case an irrecoverable error
		// is thrown during the call to Start, which terminates the calling goroutine
		go component.Start(signalingCtx)

		done = component.Done()
		return
	}

	shutdownAndWaitForRestart := func(err error) error {
		// shutdown the component
		// we need to first check if the context is canceled, because
		// if it was canceled during startup then the context's error
		// may have been thrown by ComponentManager, and it doesn't
		// make sense to pass this error to the error handler
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			cancel()
		}

		// wait until it's done
		// note that irrecoverables which are encountered during shutdown are ignored
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

		return nil
	}

	for {
		if err := start(); err != nil {
			return err // failure to start
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-irrecoverables:
			if canceled := shutdownAndWaitForRestart(err); canceled != nil {
				return canceled
			}
		}
	}
}

// ComponentWorker represents a worker routine of a component
type ComponentWorker func(ctx irrecoverable.SignalerContext)

// ComponentStartup implements a startup routine for a component
// This is where a component should perform any necessary startup
// tasks before worker routines are launched.
type ComponentStartup func(context.Context) error

// ComponentManagerBuilder provides a mechanism for building a ComponentManager
type ComponentManagerBuilder interface {
	// OnStart sets the startup routine for the ComponentManager
	OnStart(ComponentStartup) ComponentManagerBuilder

	// AddWorker adds a worker routine for the ComponentManager
	AddWorker(ComponentWorker) ComponentManagerBuilder

	// Build builds and returns a new ComponentManager instance
	Build() *ComponentManager
}

type ComponentManagerBuilderImpl struct {
	startup ComponentStartup
	workers []ComponentWorker
}

// NewComponentManagerBuilder returns a new ComponentManagerBuilder
func NewComponentManagerBuilder() ComponentManagerBuilder {
	return &ComponentManagerBuilderImpl{
		startup: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return nil
			}
		},
	}
}

func (c *ComponentManagerBuilderImpl) OnStart(startup ComponentStartup) ComponentManagerBuilder {
	c.startup = startup
	return c
}

func (c *ComponentManagerBuilderImpl) AddWorker(worker ComponentWorker) ComponentManagerBuilder {
	c.workers = append(c.workers, worker)
	return c
}

func (c *ComponentManagerBuilderImpl) Build() *ComponentManager {
	return &ComponentManager{
		started: make(chan struct{}),
		ready:   make(chan struct{}),
		startup: c.startup,
		workers: c.workers,
	}
}

var _ Component = (*ComponentManager)(nil)

// ComponentManager is used to manage worker routines of a Component
type ComponentManager struct {
	started chan struct{}
	ready   chan struct{}
	done    sync.WaitGroup

	startOnce sync.Once
	startup   func(context.Context) error
	workers   []ComponentWorker
}

// Start initiates the ComponentManager. It will first run the startup routine if one
// was set, and then launch all worker routines. If an error occurs during startup,
// it will be thrown with the provided SignalerContext.
func (c *ComponentManager) Start(ctx irrecoverable.SignalerContext) {
	c.startOnce.Do(func() {
		go func() {
			defer close(c.started)

			if err := c.startup(ctx); err != nil {
				// if startup fails, we should shut down the component immediately
				ctx.Throw(err)
			}
			close(c.ready)

			c.done.Add(len(c.workers))
			for _, worker := range c.workers {
				go func(w ComponentWorker) {
					defer c.done.Done()
					w(ctx)
				}(worker)
			}
		}()
	})
}

// Ready returns a channel which is closed once the startup routine has completed successfully.
// If an error occurs during startup, the returned channel willl never close.
func (c *ComponentManager) Ready() <-chan struct{} {
	return c.ready
}

// Done returns a channel which is closed once the component has shut down. This includes the
// scenario where an error is encountered during startup.
func (c *ComponentManager) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		// we must first wait for startup to be triggered, otherwise calling
		// Done before startup has completed will result in the returned
		// channel being closed immediately
		<-c.started
		c.done.Wait()
		close(done)
	}()
	return done
}

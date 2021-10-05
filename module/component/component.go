package component

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
)

// Component represents a component which can be started and stopped, and exposes
// channels that close when startup and shutdown have completed.
// Once Start has been called, the channel returned by Done must close eventually,
// whether that be because of a graceful shutdown or an irrecoverable error.
// If Start returns an error, the done channel should close immediately.
type Component interface {
	module.Startable
	module.ReadyDoneAware
}

type NoopComponent struct{}

var _ Component = (*NoopComponent)(nil)

func (c *NoopComponent) Start(irrecoverable.SignalerContext) error { return nil }
func (c *NoopComponent) Ready() <-chan struct{}                    { return nil }
func (c *NoopComponent) Done() <-chan struct{}                     { return nil }

type ComponentFactory func() (Component, error)

// OnError reacts to an irrecoverable error
// It is meant to inspect the error, determining its type and seeing if e.g. a restart or some other measure is suitable,
// and then return an ErrorHandlingResult indicating how RunComponent should proceed.
// Before returning, it could also:
// - panic (in canary / benchmark)
// - log in various Error channels and / or send telemetry ...
type OnError = func(err error) ErrorHandlingResult

type ErrorHandlingResult int

const (
	ErrorHandlingRestart ErrorHandlingResult = iota
	ErrorHandlingStop
)

// RunComponent repeatedly starts components returned from the given ComponentFactory, shutting them
// down when they encounter irrecoverable errors and passing those errors to the given error handler.
// If the given context is cancelled, it will wait for the current component instance to shutdown
// before returning.
// The returned error is either:
// - The context error if the context was canceled
// - The last error handled if the error handler returns ErrorHandlingStop
// - An error returned from componentFactory while generating an instance of component
// - An error returned from starting an instance of the component
func RunComponent(ctx context.Context, componentFactory ComponentFactory, handler OnError) error {
	// reference to per-run signals for the component
	var component Component
	var cancel context.CancelFunc
	var done <-chan struct{}
	var irrecoverableErr <-chan error

	start := func() error {
		var err error

		component, err = componentFactory()
		if err != nil {
			return err // failure to generate the component, should be handled out-of-band because a restart won't help
		}

		// context used to run the component
		var runCtx context.Context
		runCtx, cancel = context.WithCancel(ctx)

		// signaler used for irrecoverables
		var signaler *irrecoverable.Signaler
		signaler, irrecoverableErr = irrecoverable.NewSignaler()
		signalingCtx := irrecoverable.WithSignaler(runCtx, signaler)

		if err = component.Start(signalingCtx); err != nil {
			cancel()
			return err
		}

		done = component.Done()

		return nil
	}

	stop := func() {
		// shutdown the component
		cancel()

		// wait until it's done
		<-done
	}

	for {
		if err := start(); err != nil {
			return err // failure to start
		}

		select {
		case <-done:
			// either clean completion or canceled by context
			return ctx.Err()
		case err := <-irrecoverableErr:
			stop()

			// send error to the handler
			switch result := handler(err); result {
			case ErrorHandlingRestart:
				continue
			case ErrorHandlingStop:
				return err
			default:
				panic(fmt.Sprint("invalid error handling result: %v", result))
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
	// OnStart sets the startup routine for the ComponentManager. If an error is
	// encountered during the startup routine, the ComponentManager will shutdown
	// immediately.
	OnStart(ComponentStartup) ComponentManagerBuilder

	// AddWorker adds a worker routine for the ComponentManager
	AddWorker(ComponentWorker) ComponentManagerBuilder

	// AddComponent adds a new sub-component for the ComponentManager.
	// This should be used for critical sub-components whose failure would be
	// considered irrecoverable. For non-critical sub-components, consider using
	// RunComponent instead.
	AddComponent(Component) ComponentManagerBuilder

	// Build builds and returns a new ComponentManager instance
	Build() *ComponentManager
}

type ComponentManagerBuilderImpl struct {
	startup    ComponentStartup
	workers    []ComponentWorker
	components []Component
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

func (c *ComponentManagerBuilderImpl) AddComponent(component Component) ComponentManagerBuilder {
	c.components = append(c.components, component)
	return c
}

func (c *ComponentManagerBuilderImpl) Build() *ComponentManager {
	return &ComponentManager{
		started:    atomic.NewBool(false),
		ready:      make(chan struct{}),
		done:       make(chan struct{}),
		startup:    c.startup,
		workers:    c.workers,
		components: c.components,
	}
}

var _ Component = (*ComponentManager)(nil)

// ComponentManager is used to manage worker routines and sub-components of a Component.
// When a ComponentManager is started, it first runs the startup function if one is provided,
// and then proceeds to start up any components that it contains. Once all components have
// started up successfully, it then launches any worker routines.
type ComponentManager struct {
	started        *atomic.Bool
	ready          <-chan struct{}
	done           chan struct{}
	shutdownSignal <-chan struct{}

	startup    func(context.Context) error
	workers    []ComponentWorker
	components []Component
}

// Start initiates the ComponentManager. It will first run the startup routine if one
// was set, and then start all sub-components and launch all worker routines.
func (c *ComponentManager) Start(parent irrecoverable.SignalerContext) (err error) {
	// only start once
	if c.started.CAS(false, true) {
		ctx, cancel := context.WithCancel(parent)
		_ = cancel // pacify vet lostcancel check: startupCtx is always canceled through its parent
		defer func() {
			if err != nil {
				cancel()
			}
		}()
		c.shutdownSignal = ctx.Done()

		if err = c.startup(ctx); err != nil {
			defer close(c.done)
			return
		}

		signaler := irrecoverable.NewSignaler()
		startupCtx := irrecoverable.WithSignaler(ctx, signaler)

		go func() {
			select {
			case err := <-signaler.Error():
				cancel()

				// we propagate the error directly to the parent because a failure in a
				// worker routine or a critical sub-component is considered irrecoverable

				// TODO: It may be useful to allow the user of the ComponentManager to wrap
				// errors thrown from sub-components before propagating them to the parent,
				// to provide context for the parent about which sub-component the error
				// originates from. This way the parent error handler doesn't need to handle
				// every irrecoverable error type that may be thrown from a sub-component.
				parent.Throw(err)
			case <-c.done:
			}
		}()

		// TODO: We may eventually want to introduce a way for sub-component startup
		// and the provided ComponentStartup to occur concurrently, or even make this
		// the default.
		// In the current usecases, sub-components may have dependencies that must be
		// initialized in the provided ComponentStartup before they can be started,
		// but these dependencies can be removed by refactoring the code.

		var componentGroup errgroup.Group
		for _, component := range c.components {
			component := component
			componentGroup.Go(func() error {
				if err := component.Start(startupCtx); err != nil {
					defer cancel() // cancel startup for all other components
					return err
				}
				return nil
			})
		}

		components := make([]module.ReadyDoneAware, len(c.components))
		for i, component := range c.components {
			components[i] = component.(module.ReadyDoneAware)
		}
		componentsDone := util.AllDone(components...)

		if err = componentGroup.Wait(); err != nil {
			// wait for sub-components to shutdown before returning
			<-componentsDone

			defer close(c.done)
			return
		}

		c.ready = util.AllReady(components...)

		var workersDone sync.WaitGroup
		workersDone.Add(len(c.workers))
		for _, worker := range c.workers {
			go func(w ComponentWorker) {
				defer workersDone.Done()
				w(startupCtx)
			}(worker)
		}

		// launch goroutine to close done channel
		go func() {
			// wait for sub-components to shutdown
			<-componentsDone

			// wait for worker routines to finish
			workersDone.Wait()

			close(c.done)
		}()

		return
	}

	return module.ErrMultipleStartup
}

// Ready returns a channel which is closed once the startup routine has completed successfully.
// If an error occurs during startup, the returned channel will never close. If this is called
// before startup has been initiated, a nil channel will be returned.
func (c *ComponentManager) Ready() <-chan struct{} {
	return c.ready
}

// Done returns a channel which is closed once the ComponentManager has shut down following a
// call to Start. This includes ungraceful shutdowns, such as when an error is encountered
// during startup. If startup had succeeded, it will wait for all worker routines and sub-components
// to shut down before closing the returned channel.
func (c *ComponentManager) Done() <-chan struct{} {
	return c.done
}

// ShutdownSignal returns a channel that is closed when shutdown has commenced.
// If this is called before startup has been initiated, a nil channel will be returned.
func (c *ComponentManager) ShutdownSignal() <-chan struct{} {
	return c.shutdownSignal
}

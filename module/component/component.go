package component

import (
	"context"
	"errors"
	"sync"

	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
)

var ErrMultipleStartup = errors.New("component may only be started once")

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
	var irrecoverableErr <-chan error

	start := func() (err error) {
		component, err = componentFactory()
		if err != nil {
			return // failure to generate the component, should be handled out-of-band because a restart won't help
		}

		// context used to run the component
		var runCtx context.Context
		runCtx, cancel = context.WithCancel(ctx)

		// signaler used for irrecoverables
		signaler := irrecoverable.NewSignaler()
		signalingCtx := irrecoverable.WithSignaler(runCtx, signaler)
		irrecoverableErr = signaler.Error()

		// the component must be started in a separate goroutine in case an irrecoverable error
		// is thrown during the call to Start, which terminates the calling goroutine
		startDone := make(chan struct{})
		go func() {
			defer close(startDone)
			err = component.Start(signalingCtx)
		}()
		<-startDone
		if err != nil {
			cancel()
			return
		}

		done = component.Done()
		return
	}

	shutdownAndWaitForRestart := func(err error) error {
		// shutdown the component
		cancel()

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

		defer cancel()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-irrecoverableErr:
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
	// OnStart sets the startup routine for the ComponentManager. If an error is
	// encountered during the startup routine, the ComponentManager will shutdown
	// immediately.
	OnStart(ComponentStartup) ComponentManagerBuilder

	// AddWorker adds a worker routine for the ComponentManager
	AddWorker(ComponentWorker) ComponentManagerBuilder

	// AddComponent adds a new sub-component for the ComponentManager.
	// This should be used for critical sub-components whose failure should be
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
		started:     atomic.NewBool(false),
		startupDone: make(chan struct{}),
		ready:       make(chan struct{}),
		startup:     c.startup,
		workers:     c.workers,
		components:  c.components,
	}
}

var _ Component = (*ComponentManager)(nil)

// ComponentManager is used to manage worker routines of a Component
type ComponentManager struct {
	started        *atomic.Bool
	startupDone    chan struct{}
	ready          chan struct{}
	done           sync.WaitGroup
	shutdownSignal <-chan struct{}

	startup    func(context.Context) error
	workers    []ComponentWorker
	components []Component
}

// Start initiates the ComponentManager. It will first run the startup routine if one
// was set, and then start all sub-components and launch all worker routines.
func (c *ComponentManager) Start(parent irrecoverable.SignalerContext) (err error) {
	if c.started.CAS(false, true) {
		defer close(c.startupDone)

		ctx, cancel := context.WithCancel(parent)
		_ = cancel // pacify vet lostcancel check: startupCtx is always canceled through its parent
		defer func() {
			if err != nil {
				cancel()
			}
		}()
		c.shutdownSignal = ctx.Done()

		if err = c.startup(ctx); err != nil {
			return
		}

		signaler := irrecoverable.NewSignaler()
		startupCtx := irrecoverable.WithSignaler(ctx, signaler)

		go func() {
			select {
			case err := <-signaler.Error():
				cancel()

				// we propagate the error directly to the parent because a failure
				// in a critical sub-component is considered irrecoverable
				parent.Throw(err)
			case <-ctx.Done():
				// TODO: should we instead wait for parent.Done() or c.Done()?
			}
		}()

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

		if err = componentGroup.Wait(); err != nil {
			// TODO: should we wait for c.Done() before returning?
			return
		}

		close(c.ready)

		c.done.Add(len(c.workers))
		for _, worker := range c.workers {
			go func(w ComponentWorker) {
				defer c.done.Done()
				w(startupCtx)
			}(worker)
		}

		return
	}

	return ErrMultipleStartup
}

// Ready returns a channel which is closed once the startup routine has completed successfully.
// If an error occurs during startup, the returned channel will never close.
func (c *ComponentManager) Ready() <-chan struct{} {
	return c.ready
}

// Done returns a channel which is closed once the ComponentManager has shut down following a
// call to Start. This includes ungraceful shutdowns, such as when an error is encountered
// during startup. If startup had succeeded, it will wait for all worker routines and sub-components
// to shut down before closing the returned channel.
func (c *ComponentManager) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		// we must first wait for startup to be triggered, otherwise calling
		// Done before startup has completed will result in the returned
		// channel being closed immediately
		<-c.startupDone

		// wait for sub-components to shutdown
		components := make([]module.ReadyDoneAware, len(c.components))
		for i, component := range c.components {
			components[i] = component.(module.ReadyDoneAware)
		}
		<-util.AllDone(components...)

		// wait for worker routines to finish
		c.done.Wait()

		close(done)
	}()
	return done
}

// ShutdownSignal returns a channel that is closed when shutdown has commenced.
// If this is called before startup has been initiated, a nil channel will be returned.
func (c *ComponentManager) ShutdownSignal() <-chan struct{} {
	return c.shutdownSignal
}

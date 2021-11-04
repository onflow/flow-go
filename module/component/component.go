package component

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
)

// Component represents a component which can be started and stopped, and exposes
// channels that close when startup and shutdown have completed.
// Once Start has been called, the channel returned by Done must close eventually,
// whether that be because of a graceful shutdown or an irrecoverable error.
type Component interface {
	module.Startable
	module.ReadyDoneAware
}

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

		// signaler context used for irrecoverables
		var signalCtx irrecoverable.SignalerContext
		signalCtx, irrecoverableErr = irrecoverable.WithSignaler(runCtx)

		// we start the component in a separate goroutine, since an irrecoverable error
		// could be thrown with `signalCtx` which terminates the calling goroutine
		go component.Start(signalCtx)

		done = component.Done()

		return nil
	}

	stop := func() {
		// shutdown the component and wait until it's done
		cancel()
		<-done
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := start(); err != nil {
			return err // failure to start
		}

		doneCtx, _ := util.WithDone(ctx, done)
		if err := util.WaitError(doneCtx, irrecoverableErr); err != nil {
			// an irrecoverable error was encountered
			stop()

			// send error to the handler
			switch result := handler(err); result {
			case ErrorHandlingRestart:
				continue
			case ErrorHandlingStop:
				return err
			default:
				panic(fmt.Sprintf("invalid error handling result: %v", result))
			}
		} else if util.CheckClosed(ctx.Done()) {
			// the parent context was cancelled
			stop()
			return ctx.Err()
		}

		// clean completion
		return nil
	}
}

// ReadyFunc is called within a ComponentWorker function to indicate that the worker is ready
// ComponentManager's Ready channel is closed when all workers are ready. This is also the
// signal used to indicate that the workers returned by lookup are ready.
type ReadyFunc func()

// LookupFunc returns a ReadyDoneAware interface for the worker identified by id, and a boolean
// signalling if the worker exists.
// workerID can be prefixed with ! to get a ReadyDoneAware interface for the collection of all
// workers EXCEPT the id provided. This is particularly useful if there is a main worker that has
// shutdown logic that should only run after all other workers have shutdown.
//
// The ReadyDoneAware interface returned by this function can be used to synchronize with the
// worker associated with workerID.
// Caution: waiting on workers can easily create deadlocks. use with care
type LookupFunc func(workerID string) (module.ReadyDoneAware, bool)

// ComponentWorker represents a worker routine of a component.
// It takes a SignalerContext which can be used to throw any irrecoverable errors it encounters,
// as well as a ReadyFunc which must be called to signal that it is ready. The ComponentManager
// waits until all workers have signaled that they are ready before closing its own Ready channel.
// It additionally takes a LookupFunc which allows the worker function to get a ReadyDoneAware
// interface for any other worker routine. This provides a convenient interface for managing
// inter worker dependencies.
type ComponentWorker func(ctx irrecoverable.SignalerContext, ready ReadyFunc, lookup LookupFunc)

// ComponentManagerBuilder provides a mechanism for building a ComponentManager
type ComponentManagerBuilder interface {
	// AddWorker adds a worker routine for the ComponentManager
	AddWorker(string, ComponentWorker) ComponentManagerBuilder

	// Build builds and returns a new ComponentManager instance
	Build() *ComponentManager

	// SetSerialStart configures the ComponentManager to start workers serially
	// By default, workers are started in parallel and must have explicit dependency checks
	// This allows using the order workers were added to manage dependencies implicitly
	SetSerialStart(bool) ComponentManagerBuilder
}

type componentManagerBuilderImpl struct {
	mu              sync.Mutex
	built           bool
	serialStart     bool
	workers         []*worker
	workersRegistry map[string]module.ReadyDoneAware
}

// NewComponentManagerBuilder returns a new ComponentManagerBuilder
func NewComponentManagerBuilder() ComponentManagerBuilder {
	return &componentManagerBuilderImpl{
		mu:              sync.Mutex{},
		workersRegistry: make(map[string]module.ReadyDoneAware),
	}
}

func (c *componentManagerBuilderImpl) AddWorker(id string, f ComponentWorker) ComponentManagerBuilder {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.workersRegistry[id]; exists {
		// worker ids must be unique. adding duplicate ids will break the lookupWorker function which
		// may result in incorrect dependency ordering
		panic(fmt.Sprintf("worker with id %s already exists", id))
	}

	worker := newWorker(id, f)
	c.workers = append(c.workers, worker)
	c.workersRegistry[id] = worker
	return c
}

// SetSerialStart sets whether the workers should be started in serial or in parallel
// Workers are started in the order they were added to the ComponentManagerBuilder. If enabled,
// Start() will wait for the previous worker to call ready() before starting the next.
func (c *componentManagerBuilderImpl) SetSerialStart(serial bool) ComponentManagerBuilder {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.serialStart = serial
	return c
}

// Build returns a new ComponentManager instance with the added workers
// Build must be called exactly once. Calling it more than once may result in workers being started
// multiple times.
func (c *componentManagerBuilderImpl) Build() *ComponentManager {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.built {
		panic("component manager builder can only be used once")
	}
	c.built = true

	return &ComponentManager{
		started:         atomic.NewBool(false),
		ready:           make(chan struct{}),
		done:            make(chan struct{}),
		shutdownSignal:  make(chan struct{}),
		serialStart:     c.serialStart,
		workers:         c.workers,
		workersRegistry: c.workersRegistry,
	}
}

var _ Component = (*ComponentManager)(nil)

// ComponentManager is used to manage the worker routines of a Component
type ComponentManager struct {
	started        *atomic.Bool
	ready          chan struct{}
	done           chan struct{}
	shutdownSignal chan struct{}

	serialStart     bool
	workers         []*worker
	workersRegistry map[string]module.ReadyDoneAware
}

// Start initiates the ComponentManager by launching all worker routines.
func (c *ComponentManager) Start(parent irrecoverable.SignalerContext) {
	// Make sure we only start once. atomically check if started is false then set it to true.
	// If it was not false, panic
	if !c.started.CAS(false, true) {
		panic(module.ErrMultipleStartup)
	}

	ctx, cancel := context.WithCancel(parent)
	signalerCtx, errChan := irrecoverable.WithSignaler(ctx)
	go func() {
		<-ctx.Done()
		close(c.shutdownSignal)
	}()

	// launch goroutine to propagate irrecoverable error
	go func() {
		// only signal when done channel is closed
		doneCtx, _ := util.WithDone(context.Background(), c.done)
		if err := util.WaitError(doneCtx, errChan); err != nil {
			cancel() // shutdown all workers

			// we propagate the error directly to the parent because a failure in a
			// worker routine is considered irrecoverable
			parent.Throw(err)
		}
	}()

	// launch workers
	running := make([]module.ReadyDoneAware, len(c.workers))
	for i, worker := range c.workers {
		worker := worker
		running[i] = worker
		go worker.run(signalerCtx, c.lookupWorker)
		if c.serialStart {
			if err := util.WaitClosed(parent, worker.Ready()); err != nil {
				break
			}
		}
	}

	// launch goroutine to close ready channel
	go c.waitForReady(running)

	// launch goroutine to close done channel
	go c.waitForDone(running)
}

// lookupWorker returns a ReadyDoneAware interface for the worker specified by ID
// Prefix id with ! to get a ReadyDoneAware interface for the collection of all workers except the
// provided id.
func (c *ComponentManager) lookupWorker(id string) (module.ReadyDoneAware, bool) {
	if id[0] == '!' {
		workers := []module.ReadyDoneAware{}
		for wid, w := range c.workersRegistry {
			if wid != id[1:] {
				workers = append(workers, w)
			}
		}
		if len(workers) == 0 {
			return nil, false
		}

		return module.NewCustomReadyDoneAware(util.AllReady(workers...), util.AllDone(workers...)), true
	}

	worker, exists := c.workersRegistry[id]
	return worker, exists
}

func (c *ComponentManager) waitForReady(workers []module.ReadyDoneAware) {
	<-util.AllReady(workers...)
	close(c.ready)
}

func (c *ComponentManager) waitForDone(workers []module.ReadyDoneAware) {
	<-util.AllDone(workers...)
	close(c.done)
}

// Ready returns a channel which is closed once all the worker routines have been launched and are ready.
// If any worker routines exit before they indicate that they are ready, the channel returned from Ready will never close.
func (c *ComponentManager) Ready() <-chan struct{} {
	return c.ready
}

// Done returns a channel which is closed once the ComponentManager has shut down.
// This happens when all worker routines have shut down (either gracefully or by throwing an error).
func (c *ComponentManager) Done() <-chan struct{} {
	return c.done
}

// ShutdownSignal returns a channel that is closed when shutdown has commenced.
// This can happen either if the ComponentManager's context is canceled, or a worker routine encounters
// an irrecoverable error.
// If this is called before Start, a nil channel will be returned.
func (c *ComponentManager) ShutdownSignal() <-chan struct{} {
	return c.shutdownSignal
}

type worker struct {
	id    string
	f     ComponentWorker
	ready chan struct{}
	done  chan struct{}
}

func newWorker(id string, f ComponentWorker) *worker {
	return &worker{
		id:    id,
		f:     f,
		ready: make(chan struct{}),
		done:  make(chan struct{}),
	}
}

// run calls the worker's ComponentWorker function, and manages the ReadyDoneAware interface
func (w *worker) run(ctx irrecoverable.SignalerContext, lookup LookupFunc) {
	defer close(w.done)

	var readyOnce sync.Once
	w.f(ctx, func() {
		readyOnce.Do(func() {
			close(w.ready)
		})
	}, lookup)
}

// Ready returns a channel which is closed once all the worker routines have been launched and
// are ready.
func (w *worker) Ready() <-chan struct{} {
	return w.ready
}

// Done returns a channel which is closed once the worker has shut down.
func (w *worker) Done() <-chan struct{} {
	return w.done
}

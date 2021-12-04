package component

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
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

		select {
		case <-ctx.Done():
			stop()
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
				panic(fmt.Sprintf("invalid error handling result: %v", result))
			}
		case <-done:
			// Without this additional select, there is a race condition here where the done channel
			// could have been closed as a result of an irrecoverable error being thrown, so that when
			// the scheduler yields control back to this goroutine, both channels are available to read
			// from. If this last case happens to be chosen at random to proceed instead of the one
			// above, then we would return as if the component shutdown gracefully, when in fact it
			// encountered an irrecoverable error.
			select {
			case err := <-irrecoverableErr:
				switch result := handler(err); result {
				case ErrorHandlingRestart:
					continue
				case ErrorHandlingStop:
					return err
				default:
					panic(fmt.Sprintf("invalid error handling result: %v", result))
				}
			default:
			}

			// Similarly, the done channel could have closed as a result of the context being canceled.
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// clean completion
			return nil
		}
	}
}

type ReadyFunc func()

// ComponentWorker represents a worker routine of a component.
// It takes a SignalerContext which can be used to throw any irrecoverable errors it encounters,
// as well as a ReadyFunc which must be called to signal that it is ready. The ComponentManager
// waits until all workers have signaled that they are ready before closing its Component's
// Ready channel.
type ComponentWorker func(ctx irrecoverable.SignalerContext, ready ReadyFunc)

// ComponentTask represents a one-off task.
// It takes a SignalerContext which can be used to throw any irrecoverable errors it encounters.
type ComponentTask func(ctx irrecoverable.SignalerContext)

// ComponentManagerBuilder provides a mechanism for building a ComponentManager
type ComponentManagerBuilder interface {
	// AddWorker adds a worker routine for the ComponentManager
	AddWorker(ComponentWorker) ComponentManagerBuilder

	// Build builds and returns a new ComponentManager instance
	Build() ComponentManager
}

// ComponentManager manages the worker routines and tasks of a Component.
type ComponentManager interface {
	Component() Component

	// Run executes the given ComponentTask, passing in the context that the
	// ComponentManager's Component was started with.
	// If the Component is shutdown before the task finishes executing,
	// the returned error will be ErrComponentManagerShutdown.
	Run(task ComponentTask) error

	// RunAsync executes the given ComponentAsyncTask asynchronously, passing
	// in the context that the ComponentManager's Component was started with.
	RunAsync(task ComponentTask)

	// ShutdownSignal returns a channel that is closed when shutdown of the
	// Component has commenced.
	// This can happen either if the Component's context is canceled, or a
	// worker routine encounters an irrecoverable error.
	ShutdownSignal() <-chan struct{}
}

type componentManagerBuilderImpl struct {
	workers []ComponentWorker
}

// NewComponentManagerBuilder returns a new ComponentManagerBuilder
func NewComponentManagerBuilder() ComponentManagerBuilder {
	return &componentManagerBuilderImpl{}
}

func (c *componentManagerBuilderImpl) AddWorker(worker ComponentWorker) ComponentManagerBuilder {
	c.workers = append(c.workers, worker)
	return c
}

func (c *componentManagerBuilderImpl) Build() ComponentManager {
	shutdownSignal := make(chan struct{})

	taskRunner := &taskRunner{
		tasks:          make(chan *runTaskRequest),
		asyncTasks:     make(chan ComponentTask),
		shutdownSignal: shutdownSignal,
	}

	// copy workers to a new slice to prevent further modifications
	workers := make([]ComponentWorker, len(c.workers)+1)
	workers[0] = taskRunner.handleTasks
	copy(workers[1:], c.workers)

	return &componentManagerImpl{
		started:        atomic.NewBool(false),
		ready:          make(chan struct{}),
		done:           make(chan struct{}),
		shutdownSignal: shutdownSignal,
		workers:        workers,
		taskRunner:     taskRunner,
	}
}

var ErrComponentManagerClosed = errors.New("component manager is closed")

type taskRunner struct {
	tasks          chan *runTaskRequest
	asyncTasks     chan ComponentTask
	shutdownSignal chan struct{}
}

type runTaskRequest struct {
	task ComponentTask
	done chan struct{}
}

func (t *taskRunner) Run(task ComponentTask) error {
	done := make(chan struct{})

	select {
	case <-t.shutdownSignal:
		return ErrComponentManagerClosed
	case t.tasks <- &runTaskRequest{task, done}:
		select {
		case <-t.shutdownSignal:
			return ErrComponentManagerClosed
		case <-done:
			return nil
		}
	}
}

func (t *taskRunner) RunAsync(task ComponentTask) {
	go func() {
		select {
		case <-t.shutdownSignal:
		case t.asyncTasks <- task:
		}
	}()
}

func (t *taskRunner) handleTasks(ctx irrecoverable.SignalerContext, ready ReadyFunc) {
	var tasksDone sync.WaitGroup

	defer tasksDone.Wait()

	ready()

	for {
		select {
		case taskRequest := <-t.tasks:
			tasksDone.Add(1)

			go func() {
				defer tasksDone.Done()
				defer close(taskRequest.done)

				taskRequest.task(ctx)
			}()
		case asyncTask := <-t.asyncTasks:
			tasksDone.Add(1)

			go func() {
				defer tasksDone.Done()

				asyncTask(ctx)
			}()
		case <-ctx.Done():
			return
		}
	}
}

var _ ComponentManager = (*componentManagerImpl)(nil)

type componentManagerImpl struct {
	started        *atomic.Bool
	ready          chan struct{}
	done           chan struct{}
	shutdownSignal chan struct{}

	workers []ComponentWorker

	*taskRunner
}

func (c *componentManagerImpl) Component() Component {
	return c
}

// Start initiates the ComponentManager by launching all worker routines.
func (c *componentManagerImpl) Start(parent irrecoverable.SignalerContext) {
	// only start once
	if c.started.CAS(false, true) {
		ctx, cancel := context.WithCancel(parent)
		signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

		go c.waitForShutdownSignal(ctx.Done())

		// launch goroutine to propagate irrecoverable error
		go func() {
			select {
			case err := <-errChan:
				cancel() // shutdown all workers

				// we propagate the error directly to the parent because a failure in a
				// worker routine is considered irrecoverable
				parent.Throw(err)
			case <-c.done:
				// Without this additional select, there is a race condition here where the done channel
				// could be closed right after an irrecoverable error is thrown, so that when the scheduler
				// yields control back to this goroutine, both channels are available to read from. If this
				// second case happens to be chosen at random to proceed, then we would return and silently
				// ignore the error.
				select {
				case err := <-errChan:
					cancel()
					parent.Throw(err)
				default:
				}
			}
		}()

		var workersReady sync.WaitGroup
		var workersDone sync.WaitGroup
		workersReady.Add(len(c.workers))
		workersDone.Add(len(c.workers))

		// launch workers
		for _, worker := range c.workers {
			worker := worker
			go func() {
				defer workersDone.Done()
				var readyOnce sync.Once
				worker(signalerCtx, func() {
					readyOnce.Do(func() {
						workersReady.Done()
					})
				})
			}()
		}

		c.workers = nil

		// launch goroutine to close ready channel
		go c.waitForReady(&workersReady)

		// launch goroutine to close done channel
		go c.waitForDone(&workersDone)
	} else {
		panic(module.ErrMultipleStartup)
	}
}

func (c *componentManagerImpl) waitForShutdownSignal(shutdownSignal <-chan struct{}) {
	<-shutdownSignal
	close(c.shutdownSignal)
}

func (c *componentManagerImpl) waitForReady(workersReady *sync.WaitGroup) {
	workersReady.Wait()
	close(c.ready)
}

func (c *componentManagerImpl) waitForDone(workersDone *sync.WaitGroup) {
	workersDone.Wait()
	close(c.done)
}

// Ready returns a channel which is closed once all the worker routines have been launched and are ready.
// If any worker routines exit before they indicate that they are ready, the channel returned from Ready will never close.
func (c *componentManagerImpl) Ready() <-chan struct{} {
	return c.ready
}

// Done returns a channel which is closed once the ComponentManager has shut down.
// This happens when all worker routines have shut down (either gracefully or by throwing an error).
func (c *componentManagerImpl) Done() <-chan struct{} {
	return c.done
}

// ShutdownSignal returns a channel that is closed when shutdown has commenced.
// This can happen either if the ComponentManager's context is canceled, or a worker routine encounters
// an irrecoverable error.
func (c *componentManagerImpl) ShutdownSignal() <-chan struct{} {
	return c.shutdownSignal
}

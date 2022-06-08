# Component Interface (Core Protocol)

| Status        | Proposed                                                  |
:-------------- |:--------------------------------------------------------- |
| **FLIP #**    | [1167](https://github.com/onflow/flow-go/pull/1167)       |
| **Author(s)** | Simon Zhu (simon.zhu@dapperlabs.com)                      |
| **Sponsor**   | Simon Zhu (simon.zhu@dapperlabs.com)                      |
| **Updated**   | 9/16/2021                                                 |

## Objective

FLIP to separate the API through which components are started from the API through which they expose their status.

## Current Implementation

The [`ReadyDoneAware`](https://github.com/onflow/flow-go/blob/7763000ba5724bb03f522380e513b784b4597d46/module/common.go#L6) interface provides an interface through which components / modules can be started and stopped. Calling the `Ready` method should start the component and return a channel that will close when startup has completed, and `Done` should be the corresponding method to shut down the component.

### Potential problems 

The current `ReadyDoneAware` interface is misleading, as by the name one might expect that it is only used to check the state of a component. However, in almost all current implementations the `Ready` method is used to both start the component *and* check when it has started up, and similarly for the `Done` method. 

This introduces issues of concurrency safety / idempotency, as most implementations do not properly handle the case where the `Ready` or `Done` methods are called more than once. See [this example](https://github.com/onflow/flow-go/pull/1026).

[Clearer documentation](https://github.com/onflow/flow-go/pull/1032) and a new [`LifecycleManager`](https://github.com/onflow/flow-go/pull/1031) component were introduced as a step towards fixing this by providing concurrency-safety for components implementing `ReadyDoneAware`, but this still does not provide a clear separation between the ability to start / stop a component and the ability to check its state. A component usually only needs to be started once, whereas multiple other components may wish to check its state.

## Proposal

Moving forward, we will add a new `Startable` interface in addition to the existing `ReadyDoneAware`:
```golang
// Startable provides an interface to start a component. Once started, the component
// can be stopped by cancelling the given context.
type Startable interface {
  // Start starts the component. Any errors encountered during startup should be returned
  // directly, whereas irrecoverable errors encountered while the component is running
  // should be thrown with the given SignalerContext.
  // This method should only be called once, and subsequent calls should return ErrMultipleStartup.
  Start(irrecoverable.SignalerContext) error
}
```
Components which implement this interface are passed in a `SignalerContext` upon startup, which they can use to propagate any irrecoverable errors they encounter up to their parent via `SignalerContext.Throw`. The parent can then choose to handle these errors however they like, including restarting the component, logging the error, propagating the error to their own parent, etc.

```golang
// We define a constrained interface to provide a drop-in replacement for context.Context
// including in interfaces that compose it.
type SignalerContext interface {
  context.Context
  Throw(err error) // delegates to the signaler
  sealed()         // private, to constrain builder to using WithSignaler
}

// private, to force context derivation / WithSignaler
type signalerCtx struct {
  context.Context
  *Signaler
}

func (sc signalerCtx) sealed() {}

// the One True Way of getting a SignalerContext
func WithSignaler(parent context.Context) (SignalerContext, <-chan error) {
  sig, errChan := NewSignaler()
  return &signalerCtx{parent, sig}, errChan
}

// Signaler sends the error out.
type Signaler struct {
  errChan   chan error
  errThrown *atomic.Bool
}

func NewSignaler() (*Signaler, <-chan error) {
  errChan := make(chan error, 1)
  return &Signaler{
    errChan:   errChan,
    errThrown: atomic.NewBool(false),
  }, errChan
}

// Throw is a narrow drop-in replacement for panic, log.Fatal, log.Panic, etc
// anywhere there's something connected to the error channel. It only sends
// the first error it is called with to the error channel, there are various
// options as to how subsequent errors can be handled.
func (s *Signaler) Throw(err error) {
  defer runtime.Goexit()
  if s.errThrown.CAS(false, true) {
    s.errChan <- err
    close(s.errChan)
  } else {
    // Another thread, possibly from the same component, has already thrown
    // an irrecoverable error to this Signaler. Any subsequent irrecoverable
    // errors can either be logged or ignored, as the parent will already
    // be taking steps to remediate the first error.
  }
}
```

> For more details about `SignalerContext` and `ErrMultipleStartup`, see [#1275](https://github.com/onflow/flow-go/pull/1275) and [#1355](https://github.com/onflow/flow-go/pull/1355/).

To start a component, a `SignalerContext` must be created to start it with:

```golang
var parentCtx context.Context // this is the context for the routine which manages the component
var childComponent component.Component

ctx, cancel := context.WithCancel(parentCtx)

// create a SignalerContext and return an error channel which can be used to receive
// any irrecoverable errors thrown with the Signaler
signalerCtx, errChan := irrecoverable.WithSignaler(ctx)

// start the child component
childComponent.Start(signalerCtx)

// launch goroutine to handle errors thrown from the child component
go func() {
  select {
  case err := <-errChan: // error thrown by child component
    cancel()
    // handle the error...
  case <-parentCtx.Done(): // canceled by parent
    // perform any necessary cleanup...
  }
}
```

With all of this in place, the semantics of `ReadyDoneAware` can be redefined to only be used to check a component's state (i.e wait for startup / shutdown to complete)
```golang
type ReadyDoneAware interface {
  // Ready returns a channel that will close when component startup has completed.
  Ready() <-chan struct{}
  // Done returns a channel that will close when component shutdown has completed.
  Done() <-chan struct{}
}
```

Finally, we can define a `Component` interface which combines both of these interfaces:
```golang
type Component interface {
  Startable
  ReadyDoneAware
}
```

A component will now be started by passing a `SignalerContext` to its `Start` method, and can be stopped by cancelling the `Context`. If a component needs to startup subcomponents, it can create child `Context`s from this `Context` and pass those to the subcomponents.
### Motivations
- `Context`s are the standard way of doing go-routine lifecycle management in Go, and adhering to standards helps eliminate confusion and ambiguity for anyone interacting with the `flow-go` codebase. This is especially true now that we are beginning to provide API's and interfaces for third parties to interact with the codebase (e.g DPS).
  - Even to someone unfamiliar with our codebase (but familiar with Go idioms), it is clear how a method signature like `Start(context.Context) error` will behave. A method signature like `Ready()` is not so clear.
- This promotes a hierarchical supervision paradigm, where each `Component` is equipped with a fresh signaler to its parent at launch, and is thus supervised by his parent for any irrecoverable errors it may encounter (the call to `WithSignaler` replaces the signaler in a parent context). As a consequence, sub-components themselves started by a component have it as a supervisor, which handles their irrecoverable failures, and so on.
  - If context propagation is done properly, there is no need to worry about any cleanup code in the `Done` method. Cancelling the context for a component will automatically cancel all subcomponents / child routines in the component tree, and we do not have to explicitly call `Done` on each and every subcomponent to trigger their shutdown.
  - This allows us to separate the capability to check a component's state from the capability to start / stop it. We may want to give multiple other components the capability to check its state, without giving them the capability to start or stop it. Here is an [example](https://github.com/onflow/flow-go/blob/b50f0ffe054103a82e4aa9e0c9e4610c2cbf2cc9/engine/common/splitter/network/network.go#L112) of where this would be useful.
  - This provides a clearer way of defining ownership of components, and hence may potentially eliminate the need to deal with concurrency-safety altogether. Whoever creates a component should be responsible for starting it, and therefore they should be the only one with access to its `Startable` interface. If each component only has a single parent that is capable of starting it, then we should never run into concurrency issues.

## Implementation (WIP)
* Lifecycle management logic for components can be further abstracted into a `RunComponent` helper function:

  ```golang
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

      component.Start(signalCtx)

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
  ```

  > Note: this is now implemented in [#1275](https://github.com/onflow/flow-go/pull/1275) and [#1355](https://github.com/onflow/flow-go/pull/1355), and an example can be found [here](https://github.com/onflow/flow-go/blob/24406ed3fde7661cb1df84a25755cedf041a1c50/module/irrecoverable/irrecoverable_example_test.go).
* We may be able to encapsulate a lot of the boilerplate code involved in handling startup / shutdown of worker routines into a single `ComponentManager` struct:

  ```golang
  type ReadyFunc func()

  // ComponentWorker represents a worker routine of a component
  type ComponentWorker func(ctx irrecoverable.SignalerContext, ready ReadyFunc)

  // ComponentManagerBuilder provides a mechanism for building a ComponentManager
  type ComponentManagerBuilder interface {
    // AddWorker adds a worker routine for the ComponentManager
    AddWorker(ComponentWorker) ComponentManagerBuilder

    // Build builds and returns a new ComponentManager instance
    Build() *ComponentManager
  }

  // ComponentManager is used to manage the worker routines of a Component
  type ComponentManager struct {
    started        *atomic.Bool
    ready          chan struct{}
    done           chan struct{}
    shutdownSignal <-chan struct{}

    workers []ComponentWorker
  }

  // Start initiates the ComponentManager by launching all worker routines.
  func (c *ComponentManager) Start(parent irrecoverable.SignalerContext) {
    // only start once
    if c.started.CAS(false, true) {
      ctx, cancel := context.WithCancel(parent)
      signalerCtx, errChan := irrecoverable.WithSignaler(ctx)
      c.shutdownSignal = ctx.Done()

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

      // launch goroutine to close ready channel
      go c.waitForReady(&workersReady)

      // launch goroutine to close done channel
      go c.waitForDone(&workersDone)
    } else {
      panic(module.ErrMultipleStartup)
    }
  }

  func (c *ComponentManager) waitForReady(workersReady *sync.WaitGroup) {
    workersReady.Wait()
    close(c.ready)
  }

  func (c *ComponentManager) waitForDone(workersDone *sync.WaitGroup) {
    workersDone.Wait()
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
  ```

  Components that want to implement `Component` can use this `ComponentManager` to simplify implementation:

  ```golang
  type FooComponent struct {
    *component.ComponentManager
  }

  func NewFooComponent(foo fooType) *FooComponent {
    f := &FooComponent{}

    cmb := component.NewComponentManagerBuilder().
      AddWorker(f.childRoutine).
      AddWorker(f.childRoutineWithFooParameter(foo))

    f.ComponentManager = cmb.Build()

    return f
  }

  func (f *FooComponent) childRoutine(ctx irrecoverable.SignalerContext) {
    for {
      select {
      case <-ctx.Done():
        return
      default:
        // do work...
      }
    }
  }

  func (f *FooComponent) childRoutineWithFooParameter(foo fooType) component.ComponentWorker {
    return func(ctx irrecoverable.SignalerContext) {
      for {
        select {
        case <-ctx.Done():
          return
        default:
          // do work with foo...

          // encounter irrecoverable error
          ctx.Throw(errors.New("fatal error!"))
        }
      }
    }
  }
  ```

  > Note: this is now implemented in [#1355](https://github.com/onflow/flow-go/pull/1355)

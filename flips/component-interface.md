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
}

// Signaler sends the error out.
type Signaler struct {
	errChan   chan error
	errThrown *atomic.Bool
}

func NewSignaler() *Signaler {
	return &Signaler{
		errChan:   make(chan error, 1),
		errThrown: atomic.NewBool(false),
	}
}

// Error returns the Signaler's error channel.
func (s *Signaler) Error() <-chan error {
	return s.errChan
}

// Throw is a narrow drop-in replacement for panic, log.Fatal, log.Panic, etc
// anywhere there's something connected to the error channel. It only sends
// the first error it is called with to the error channel, and there are various
// options as to how subsequent errors can be handled.
func (s *Signaler) Throw(err error) {
	defer runtime.Goexit()

  // We only propagate the first irrecoverable error to the parent
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

To start a child component, a parent can first create a `SignalerContext` from its own context to pass down to the child:

```golang
ctx, cancel := context.WithCancel(parentCtx)
signaler := irrecoverable.NewSignaler()
signalerCtx irrecoverable.WithSignaler(ctx, signaler)

go func() {
  if err := childComponent.Start(signalerCtx); err != nil {
    cancel()
    // handle the error if necessary...
  }
}()

go func() {
  defer cancel()

  select {
  case <-signaler.Error():
    // handle the error
  case <-parentCtx.Done():
    // canceled by parent
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
  - If context propagation is done properly, there is no need to worry about any cleanup code in the `Done` method. Cancelling the context for a component will automatically cancel all subcomponents / child routines in the component tree, and we do not have to explicitly call `Done` on each and every subcomponent to trigger their shutdown.
- This allows us to separate the capability to check a component's state from the capability to start / stop it. We may want to give multiple other components the capability to check its state, without giving them the capability to start or stop it. Here is an [example](https://github.com/onflow/flow-go/blob/b50f0ffe054103a82e4aa9e0c9e4610c2cbf2cc9/engine/common/splitter/network/network.go#L112) of where this would be useful.
- This provides a clearer way of defining ownership of components, and hence may potentially eliminate the need to deal with concurrency-safety altogether. Whoever creates a component should be responsible for starting it, and therefore they should be the only one with access to its `Startable` interface. If each component only has a single parent that is capable of starting it, then we should never run into concurrency issues.

## Implementation (WIP)

* We may be able to encapsulate a lot of the boilerplate code involved in handling startup / shutdown of child routines / sub-components into a single `ComponentManager` struct:

  ```golang
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
  ```

  Components that want to implement `Component` can use this `ComponentManager` to simplify implementation:

  ```golang
  type FooComponent struct {
    *component.ComponentManager
  }

  func NewFooComponent(foo fooType, bar barType) *FooComponent {
    f := &FooComponent{}

    cmb := component.NewComponentManagerBuilder().
      OnStart(func(ctx context.Context) error {
        // perform startup tasks...
      }).
      AddWorker(f.childRoutine).
      AddWorker(f.childRoutineWithFooParameter(foo)).
      AddComponent(NewBarComponent(bar))

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

  > Note: this is now implemented in [#1275](https://github.com/onflow/flow-go/pull/1275)

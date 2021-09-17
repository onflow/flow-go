# Component Interface (Core Protocol)

| Status        | Proposed                                                  |
:-------------- |:--------------------------------------------------------- |
| **FLIP #**    | [1167](https://github.com/onflow/flow-go/pull/1167)       |
| **Author(s)** | Simon Zhu (simon.zhu@dapperlabs.com)                      |
| **Sponsor**   | Simon Zhu (simon.zhu@dapperlabs.com)                      |
| **Updated**   | 9/16/2021                                                 |

## Objective

FLIP to separate the API through which components are started from the API through which
they expose their status, and introduce a standard interface for handling irrecoverable errors that occur within a component.

## Current Implementation

The [`ReadyDoneAware`](https://github.com/onflow/flow-go/blob/7763000ba5724bb03f522380e513b784b4597d46/module/common.go#L6) interface provides an interface through which components / modules can be started and stopped. Calling the `Ready` method should start the component and return a channel that will close when startup has completed, and `Done` should be the corresponding method to shut down the component.

### Potential problems 

The current `ReadyDoneAware` interface is misleading, as by the name one might expect that it is only used to check the state of a component. However, in almost all current implementations the `Ready` method is used to both start the component *and* check when it has started up, and similarly for the `Done` method. 

This introduces issues of concurrency safety / idempotency, as most implementations do not properly handle the case where the `Ready` or `Done` methods are called more than once. See [this example](https://github.com/onflow/flow-go/pull/1026).

[Clearer documentation](https://github.com/onflow/flow-go/pull/1032) and a new [`LifecycleManager`](https://github.com/onflow/flow-go/pull/1031) component were introduced as a step towards fixing this by providing concurrency-safety for components implementing `ReadyDoneAware`, but this still does not provide a clear separation between the ability to start / stop a component and the ability to check its state. A component usually only needs to be started once, whereas multiple other components may wish to check its state.

Also, there currently does not exist any mechanism to handle fatal errors within a component. We want to enable a way for such errors to be handled by the parent of the component (e.g restart the component, propagate the error further up the stack by throwing a fatal error itself) rather than crashing the entire program.

## Proposal

Moving forward, we will add two new interfaces in addition to the existing `ReadyDoneAware`:
- A `Startable` interface which can be used to start and stop a component:
  ```golang
  type Startable interface {
    // Start will start the component or return an error. If the component is started successfully,
    // it can be stopped by cancelling the provided Context.
    Start(context.Context) error
  }
  ```
- An `ErrorAware` interface which a component uses to expose any irrecoverable errors it encounters:
  ```golang
  type ErrorAware interface {
    // Errors returns a channel which receives any irrecoverable errors encountered by the component.
    Errors() <-chan error
  }
  ```

Then, the semantics of `ReadyDoneAware` can be redefined to only be used to check a component's state (i.e wait for startup / shutdown to complete)
```golang
type ReadyDoneAware interface {
  // Ready returns a channel that will close when component startup has completed.
  Ready() <-chan struct{}
  // Done returns a channel that will close when component shutdown has completed.
  Done() <-chan struct{}
}
```

Finally, we can define a `Component` interface which combines all of the three interfaces above:
```golang
type Component interface {
  Startable
  ReadyDoneAware
  ErrorAware
}
```

A component will now be started by passing a `Context` to its `Start` method, and can be stopped by cancelling the `Context`. If a component needs to startup subcomponents, it can create child `Context`s from this `Context` and pass those to the subcomponents.
### Motivations
- `Context`s are the standard way of doing go-routine lifecycle management in Go, and adhering to standards helps eliminate confusion and ambiguity for anyone interacting with the `flow-go` codebase. This is especially true now that we are beginning to provide API's and interfaces for third parties to interact with the codebase (e.g DPS).
  - Even to someone unfamiliar with our codebase (but familiar with Go idioms), it is clear how a method signature like `Start(context.Context) error` will behave. A method signature like `Ready()` is not so clear.
  - If context propagation is done properly, there is no need to worry about any cleanup code in the `Done` method.
- This allows us to separate the capability to check a component's state from the capability to start / stop it. We may want to give multiple other components the capability to check its state, without giving them the capability to start or stop it. Here is an [example](https://github.com/onflow/flow-go/blob/b50f0ffe054103a82e4aa9e0c9e4610c2cbf2cc9/engine/common/splitter/network/network.go#L112) of where this would be useful.
- This provides a clearer way of defining ownership of components, and hence may potentially eliminate the need to deal with concurrency-safety altogether. Whoever creates a component should be responsible for starting it, and therefore they should be the only one with access to its `Startable` interface. If each component only has a single parent that is capable of starting it, then we should never run into concurrency issues.

## Implementation (WIP)

* We may be able to encapsulate a lot of the boilerplate code involved in handling startup / shutdown of child routines / sub-components into a single `ComponentManager` struct:

  ```golang
  type ComponentManager struct {
    subComponents []Component
    childRoutines []func(context.Context)

    childRoutinesStarted  sync.WaitGroup
    childRoutinesFinished sync.WaitGroup
  }

  type ComponentManagerBuilder interface {
    AddSubComponent(Component) ComponentManagerBuilder
    AddChildRoutine(func(context.Context)) ComponentManagerBuilder
    Build() *ComponentManager
  }

  func (c *ComponentManager) Start(ctx context.Context) error {
    for _, component := range c.subComponents {
      if err := component.Start(ctx); err != nil {
        return err
      }
    }

    for _, routine := range c.childRoutines {
      c.childRoutinesStarted.Add(1)
      c.childRoutinesFinished.Add(1)

      go func(routine func(context.Context)) {
        c.childRoutinesStarted.Done()
        defer c.childRoutinesFinished.Done()

        routine(ctx)
      }(routine)
    }
  }

  func (c *ComponentManager) Ready() <-chan struct{} {
    ready := make(chan struct{})

    go func() {
      <-lifecycle.AllReady(c.subComponents...)
      c.childRoutinesStarted.Wait()
      close(ready)
    }()

    return ready
  }

  func (c *ComponentManager) Done() <-chan struct{} {
    done := make(chan struct{})

    go func() {
      <-lifecycle.AllDone(c.subComponents...)
      c.childRoutinesFinished.Wait()
      close(done)
    }()

    return done
  }
  ```

  Components that want to implement `Component` can use this `ComponentManager` to simplify implementation:

  ```golang
  type FooComponent struct {
    *ComponentManager
  }

  func NewFooComponent(foo fooType, bar barType) *FooComponent {
    f := &FooComponent{}

    cmb := NewComponentManagerBuilder().
      AddChildRoutine(f.childRoutine).
      AddChildRoutine(f.childRoutineWithFooParameter(foo)).
      AddSubComponent(NewBarComponent(bar))

    f.ComponentManager = cmb.Build()

    return f
  }

  func (f *FooComponent) childRoutine(ctx context.Context) {
    for {
      select {
      case <-ctx.Done():
        return
      default:
        // do work...
      }
    }
  }

  func (f *FooComponent) childRoutineWithFooParameter(foo fooType) func(context.Context) {
    return func(ctx context.Context) {
      for {
        select {
        case <-ctx.Done():
          return
        default:
          // do work with foo...
        }
      }
    }
  }
  ```

* `ErrorAware` can be implemented with the help of an `ErrorManager` struct:

  ```golang
  // ErrorManager implements the ErrorAware interface, and provides a way for components
  // to signal an irrecoverable error.
  type ErrorManager struct {
    errors chan error
  }

  func NewErrorManager() *ErrorManager {
    return &ErrorManager{make(chan error)}
  }

  func (e *ErrorManager) Errors() <-chan error {
    return e.errors
  }

  func (e *ErrorManager) ThrowError(err error) {
    e.errors <- err
    runtime.Goexit()
  }
  ```

  Error handling for components can be implemented like so:

  ```golang
  type ErrorHandler func(ctx context.Context, errors <-chan error, restart func())

  type ComponentFactory func() (Component, error)

  func RunComponent(ctx context.Context, componentFactory ComponentFactory, errorHandler ErrorHandler) error {
    restartChan := make(chan struct{})

    start := func() (context.CancelFunc, <-chan struct{}, error) {
      component, err := componentFactory()
      if err != nil {
        // failed to create component
        return nil, nil, err
      }

      // context used to restart the component
      runCtx, cancel := context.WithCancel(ctx)
      if err := component.Start(runCtx); err != nil {
        // failed to start component
        cancel()
        return nil, nil, err
      }

      select {
      case <-ctx.Done():
        return nil, nil, ctx.Err()
      case <-component.Ready():
      }

      go errorHandler(runCtx, component.Errors(), func() {
        restartChan <- struct{}{}
        runtime.Goexit()
      })

      return cancel, component.Done(), nil
    }

    for {
      cancel, done, err := start()
      if err != nil {
        return err
      }

      select {
      case <-ctx.Done():
        return ctx.Err()
      case <-restartChan:
        // shutdown the component
        cancel()
      }

      select {
      case <-ctx.Done():
        return ctx.Err()
      case <-done:
      }
    }
  }
  ```
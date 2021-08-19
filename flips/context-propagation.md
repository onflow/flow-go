# Context Propagation and Network Layer Refactor (Core Protocol)



## Objective

FLIP to separate the API through which components are started from the API through which
they expose their current status. 

Also refactor the networking layer and to make it agnostic to Flow IDs.

## Current Implementation

The [`ReadyDoneAware`](https://github.com/onflow/flow-go/blob/7763000ba5724bb03f522380e513b784b4597d46/module/common.go#L6) interface provides an interface through which components / modules can be started and stopped. Calling the `Ready` method should start the component and return a channel that will close when startup has completed, and `Done` should be the corresponding method to shut down the component.

When the network layer receives a message, it will pass the message to the [`Engine`](https://github.com/onflow/flow-go/blob/7763000ba5724bb03f522380e513b784b4597d46/network/engine.go) registered on
the corresponding channel by [calling the engine's `Process` method](https://github.com/onflow/flow-go/blob/d31fd63eb651ed9faf0f677e9934baef6c4d9792/network/p2p/network.go#L406), passing it the Flow ID of the message sender.

### Potential problems 
#### Non-idempotency of `ReadyDoneAware`
The current `ReadyDoneAware` interface is misleading, as by the name one might expect that it is only used to check the state of a component. However, in almost all current implementations the `Ready` method is used to both start the component *and* check when it has started up, and similarly for the `Done` method. 

This introduces issues of concurrency safety, as most implementations do not properly handle the case where the `Ready` or `Done` methods are called more than once. See [this example](https://github.com/onflow/flow-go/pull/1026).

[Clearer documentation](https://github.com/onflow/flow-go/pull/1032) and a new [`LifecycleManager`](https://github.com/onflow/flow-go/pull/1031) component were introduced as a step towards fixing this by providing concurrency-safety for components implementing `ReadyDoneAware`, but this still does not provide a clear separation between the ability to start / stop a component and the ability to check its state. A component usually only needs to be started once, whereas multiple other components may wish to check its state.

#### Tight coupling of network layer to Flow-specific data types
The current network layer API was designed with the assumption that all messages sent and received either target or originate from staked Flow nodes. This is why an engine's [`Process`](https://github.com/onflow/flow-go/blob/master/network/engine.go#L28) method accepts a Flow ID identifying the message sender, and outgoing messages [must specify Flow ID(s)](https://github.com/onflow/flow-go/blob/master/network/conduit.go#L62) as targets.

This assumption is no longer true today. The access node, for example, may communicate with multiple (unstaked) consensus followers. It's perceivable that in the future there will be even more cases where communication with unstaked parties may happen (for example, execution nodes talking to DPS).

Furthermore, many engines actually don't need to know the `OriginID` of a message at all. For example, the synchronization engine only needs to know the origin of a sync request so that it can send back a response. However, the more direct form of this information is the peer ID of the libp2p node.

Finally, as of today the network layer does not actually verify the [`OriginID`](https://github.com/onflow/flow-go/blob/2641774451dcc1e147843a74265afb58466b50ab/network/message/message.pb.go#L29) field of messages received from the network. This opens the door to impersonation attacks, as a node can send a message pretending to be any other node simply by putting the other node's `OriginID` field inside the message they send.

## Proposal

### Separate lifecycle management from state observation

Moving forward, we should split the `ReadyDoneAware` interface into two separate interfaces:
- A `Runnable` interface which can be used to start and stop a component:
  ```golang
  type Runnable interface {
    // Run will start the component and returns immediately.
    Run(context.Context)
  }
  ```
- A `ReadyDoneAware` interface with methods `Ready` and `Done` which can be used to check a component's state (i.e wait for startup / shutdown to complete)
  ```golang
  type ReadyDoneAware interface {
    // Ready returns a channel that will close when component startup has completed.
    Ready() <-chan struct{}
    // Done returns a channel that will close when component shutdown has completed.
    Done() <-chan struct{}
  }
  ```

A component will now be started by passing a `Context` to its `Run` method, and can be stopped by cancelling the `Context`. If a component needs to startup subcomponents, it can create child `Context`s from this `Context` and pass those to the subcomponents.
#### Motivations
- `Context`s are the standard way of doing go-routine lifecycle management in Go, and adhering to standards helps eliminate confusion and ambiguity for anyone interacting with the `flow-go` codebase. This is especially true now that we are beginning to provide API's and interfaces for third parties to interact with the codebase (e.g DPS).
  - Even to someone unfamiliar with our codebase (but familiar with Go idioms), it is clear how a method signature like `Run(context.Context)` will behave. A method signature like `Ready()` is not so clear.
  - If context propagation is done properly, there is no need to worry about any cleanup code in the `Done` method.
- This allows us to separate the capability to check a component's state from the capability to start / stop it. We may want to give multiple other components the capability to check its state, without giving them the capability to start or stop it. Here is an [example](https://github.com/onflow/flow-go/blob/b50f0ffe054103a82e4aa9e0c9e4610c2cbf2cc9/engine/common/splitter/network/network.go#L112) of where this would be useful.
- This provides a clearer way of defining ownership of components, and hence may potentially eliminate the need to deal with concurrency-safety altogether. Whoever creates a component should be responsible for starting it, and therefore they should be the only one with access to its `Runnable` interface. If each component only has a single parent that is capable of starting it, then we should never run into concurrency issues.
- This enables us to define components which startup upon creation, and don't need to be explicitly `Run()` at a later point in time. Such components would only implement the `ReadyDoneAware` interface.

### Remove Flow IDs from the network layer and update the `Engine` interface

We should remove the `OriginID` field from the [`Message`](https://github.com/onflow/flow-go/blob/2641774451dcc1e147843a74265afb58466b50ab/network/message/message.pb.go#L26) protobuf entirely, and the `Engine` API should be modified so that the [`Process`](https://github.com/onflow/flow-go/blob/master/network/engine.go#L28) and [`Submit`](https://github.com/onflow/flow-go/blob/master/network/engine.go#L20) methods receive a `Context` as the first argument, and a peer ID instead of an origin Flow ID. 

```golang
type Engine interface {
	Submit(ctx context.Context, channel Channel, senderPeerID peer.ID, event interface{})
	Process(ctx context.Context, channel Channel, senderPeerID peer.ID, event interface{}) error
}
```

This `Context` should be passed to any goroutines that are launched as a result of processing the incoming message, and will contain the Flow ID of the sender as a [value](https://pkg.go.dev/context#WithValue) *if* the sender is a staked Flow node.

> While in general the use of the `Context.Value` API is somewhat contentious in the Go community, one of the few proper usecases that are widely agreed upon is for [request-scoped data](https://pkg.go.dev/context#:~:text=val.-,use%20context%20values%20only%20for%20request-scoped%20data%20that%20transits%20processes%20and%20apis%2C%20not%20for%20passing%20optional%20parameters%20to%20functions.,-The). In other words, this is a case where `Context.Value` is being used the way it was intended.

The API for [sending messages](https://github.com/onflow/flow-go/blob/b50f0ffe054103a82e4aa9e0c9e4610c2cbf2cc9/network/conduit.go#L49-L77) to the network should also be updated to accept peer IDs instead of Flow IDs as targets.

```golang
type Conduit interface {
	Publish(event interface{}, targetIDs ...peer.ID) error
	Unicast(event interface{}, targetID peer.ID) error
	Multicast(event interface{}, num uint, targetIDs ...peer.ID) error
}
```

Finally, we should either deprecate or modify the behavior of [`engine.Unit`](https://github.com/onflow/flow-go/blob/master/engine/unit.go).

#### Motivations
- The `OriginID` field really provides no use. The value of this field is set by the sender of the message, so we certainly cannot rely on its correctness. We can validate it by [checking it against the peer ID of the sender](https://github.com/onflow/flow-go/pull/1163), but this is actually unnecessary since the peer ID is enough to tell us who the sender is. We can extract the sender's network key from their peer ID, and if a downstream engine needs to know the Flow ID corresponding to this network key, we can determine this from the protocol state. If we cannot find a corresponding Flow ID, then the sender is not a staked node.
- This gives the network layer the ability to cancel the processing of a network message. This can be leveraged to implement timeouts, but may also be useful for other situations. For example, if the network layer becomes aware that a certain peer has become unreachable, it can cancel the processing of any sync requests from that peer.
- While `engine.Unit` provides some useful functionalities, it also uses the anti-pattern of [storing a `Context` inside a struct](https://github.com/onflow/flow-go/blob/b50f0ffe054103a82e4aa9e0c9e4610c2cbf2cc9/engine/unit.go#L117), something which is [specifically advised against](https://pkg.go.dev/context#:~:text=Do%20not%20store%20Contexts%20inside%20a%20struct%20type%3B%20instead%2C%20pass%20a%20Context%20explicitly%20to%20each%20function%20that%20needs%20it.%20The%20Context%20should%20be%20the%20first%20parameter%2C%20typically%20named%20ctx%3A) by [the developers of Go](https://go.dev/blog/context-and-structs#TOC_2.). Here is an [example](https://go.dev/blog/context-and-structs#:~:text=Storing%20context%20in%20structs%20leads%20to%20confusion) illustrating some of the problems with this approach. Instead, the same functionalities can probably be implemented using stateless functions which take a `Context` as a parameter.

## Implementation Steps (unfinished)

1. TODO
### Step 1: 
TODO

Notes:
* For channels where there should only be staked node participants, we can register a [topic validator](https://pkg.go.dev/github.com/libp2p/go-libp2p-pubsub#PubSub.RegisterTopicValidator) which validates the sender of each incoming message by extracting their network key from their peer ID and looking it up in the protocol state. If the peer ID is valid, we can take the corresponding Flow ID of the sender and add it to the `Context` which is passed to the downstream engine. Otherwise, we reject the message.
* We may be able to encapsulate a lot of the boilerplate code involved in handling startup / shutdown of child routines / sub-components into a single `BaseComponent` which implements all the logic of `Run`, `Ready`, and `Done`:

  ```golang
  type Component interface {
    ReadyDoneAware
    Runnable
  }

  type BaseComponent struct {
    SubComponents []Component
    ChildRoutines []func(context.Context)
    Cancel        context.CancelFunc

    childRoutinesStarted  sync.WaitGroup
    childRoutinesFinished sync.WaitGroup
  }

  func (c *BaseComponent) Run(parent context.Context) {
    ctx, cancel := context.WithCancel(parent)
    c.Cancel = cancel

    for _, component := range c.SubComponents {
      component.Run(ctx)
    }

    for _, routine := range c.ChildRoutines {
      c.childRoutinesStarted.Add(1)
      c.childRoutinesFinished.Add(1)

      go func(routine func(context.Context)) {
        c.childRoutinesStarted.Done()
        routine(ctx)
        c.childRoutinesFinished.Done()
      }(routine)
    }
  }

  func (c *BaseComponent) Ready() <-chan struct{} {
    ready := make(chan struct{})

    go func() {
      <-lifecycle.AllReady(c.SubComponents...)
      c.childRoutinesStarted.Wait()
      close(ready)
    }()

    return ready
  }

  func (c *BaseComponent) Done() <-chan struct{} {
    done := make(chan struct{})

    go func() {
      <-lifecycle.AllDone(c.SubComponents...)
      c.childRoutinesFinished.Wait()
      close(done)
    }()

    return done
  }
  ```

  Components that want to provide these interfaces can use this `BaseComponent` to simplify implementation:

  ```golang
  type FooComponent struct {
    *BaseComponent
  }

  func NewFooComponent(foo fooType, bar barType) *FooComponent {
    f := &FooComponent{}

    f.ChildRoutines = append(f.ChildRoutines, f.childRoutine, f.childRoutineWithFooParameter(foo))
    f.SubComponents = append(f.SubComponents, NewBarComponent(bar))

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

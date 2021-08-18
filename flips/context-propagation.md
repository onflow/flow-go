# Context Propagation and Network Layer Refactor (Core Protocol)

| Status        | Proposed                                       |
:-------------- |:---------------------------------------------- |
| **FLIP #**    | [xxx](https://github.com/onflow/flow/pull/xxx) |
| **Author(s)** | Simon Zhu (simon.zhu@dapperlabs.com)           |
| **Sponsor**   | Simon Zhu (simon.zhu@dapperlabs.com)           |
| **Updated**   | 2021-xx-xx                                     |


## Objective

FLIP to separate the API through which components are started from the API through which
they expose their current status. 

Also refactor the networking layer and to make it agnostic to Flow IDs.

## Current Implementation

The [`ReadyDoneAware`](https://github.com/onflow/flow-go/blob/7763000ba5724bb03f522380e513b784b4597d46/module/common.go#L6) interface provides an interface through which components / modules can be started and stopped. Calling the `Ready` method should start the component and return a channel that will close when startup has completed, and `Done` should be the corresponding method to shut down the component.

When the network layer receives a message, it will pass the message to the [`Engine`](https://github.com/onflow/flow-go/blob/7763000ba5724bb03f522380e513b784b4597d46/network/engine.go) registered on
the corresponding channel by [calling the engine's `Process` method](https://github.com/onflow/flow-go/blob/d31fd63eb651ed9faf0f677e9934baef6c4d9792/network/p2p/network.go#L406), passing it the Flow ID of the message sender.

#### Potential problems 

The current `ReadyDoneAware` interface is misleading, as by the name one might expect that it is only used to check the state of a component. However, in almost all current implementations the `Ready` method is used to both start the component *and* check when it has started up, and similarly for the `Done` method. 

This introduces issues of concurrency safety, as most implementations do not properly handle the case where the `Ready` or `Done` methods are called multiple times. See [this example](https://github.com/onflow/flow-go/pull/1026).

[Clearer documentation](https://github.com/onflow/flow-go/pull/1032) and a new [`LifecycleManager`](https://github.com/onflow/flow-go/pull/1031) component were introduced as a step towards fixing this by providing concurrency-safety for components implementing `ReadyDoneAware`, but this still does not provide a clear separation between the ability to start / stop a component and the ability to check its state. A component usually only needs to be started once, whereas multiple other components may wish to check its state.

The current network layer API was designed with the assumption that all messages sent and received either target or originate from staked Flow nodes. This is why an engine's [`Process`](https://github.com/onflow/flow-go/blob/master/network/engine.go#L28) method accepts a Flow ID identifying the message sender, and outgoing messages [must specify Flow ID(s)](https://github.com/onflow/flow-go/blob/master/network/conduit.go#L62) as targets.

This assumption is no longer true today. The access node, for example, may communicate with multiple (unstaked) consensus followers. It's perceivable that in the future there will be even more cases where communication with unstaked parties may happen (for example, execution nodes talking to DPS).

Furthermore, many engines actually don't need to know the `OriginID` of a message at all. For example, the synchronization engine only needs to know the origin of a sync request so that it can send back a response. However, the more direct form of this information is the peer ID of the libp2p node.

Finally, as of today the network layer does not actually verify the [`OriginID`](https://github.com/onflow/flow-go/blob/2641774451dcc1e147843a74265afb58466b50ab/network/message/message.pb.go#L29) field of messages received from the network. This opens the door to impersonation attacks, as a node can send a message pretending to be any other node simply by putting the other node's `OriginID` field inside the message they send.

## Design Proposal

Moving forward, we should split the `ReadyDoneAware` interface into two separate interfaces:
- A `Runnable` interface which can be used to start and stop a component:
  ```golang
  type Runnable interface {
    // Run will start the component
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

Additionally, the `Engine` API should be modified so that the [`Process`](https://github.com/onflow/flow-go/blob/master/network/engine.go#L28) and [`Submit`](https://github.com/onflow/flow-go/blob/master/network/engine.go#L20) methods receive a `Context` instead of an origin ID, and this `Context` should be passed to any goroutines that are launched as a result of processing incoming messages. This `Context` will always contain the peer ID of the origin libp2p node as a [value](https://pkg.go.dev/context#WithValue), and if applicable it will also contain the corresponding Flow ID.

> While in general the use of the `Context.Value` API is somewhat contentious in the Go community, one of the few proper usecases that are widely agreed upon is for [request-scoped data](https://pkg.go.dev/context#:~:text=val.-,use%20context%20values%20only%20for%20request-scoped%20data%20that%20transits%20processes%20and%20apis%2C%20not%20for%20passing%20optional%20parameters%20to%20functions.,-The). In other words, this is a case where `Context.Value` is being used the way it was intended.

We will also need to either deprecate or modify the behavior of [`engine.Unit`](https://github.com/onflow/flow-go/blob/master/engine/unit.go).

#### Motivations
- `Context`s are the standard way of doing go-routine lifecycle management in Go, and it would be beneficial to adhere to standards in order to eliminate confusion and ambiguity for anyone interacting with the API's or interfaces in the `flow-go` codebase. This is especially true now that we are beginning to provide API's for third parties to interact with the codebase (e.g DPS).
  - Even to someone unfamiliar with our codebase (but familiar with Go idioms), it is clear how a method signature like `Run(context.Context)` will behave. A method signature like `Ready()` is not so clear.
  - If context propagation is done properly, there is no need to worry about any cleanup code in the `Done` method.

  
- Allows components which startup upon creation.
- It allows us to have a clearer way of defining ownership of subcomponents, and hence may potentially eliminate the need to deal with concurrency safety altogether. The owner of a component should be the only one with access to its `Runnable` interface, and hence the only one capable of starting it.
- It allows us to separate the capability of checking a component's state with the capability of starting / stopping it. We may want to give component A the permission to do the former on component B, but not the latter. Here is an [example](https://github.com/onflow/flow-go/blob/master/engine/common/splitter/network/network.go#L112) of where this would be useful.

Once this is in place, we would be able to encapsulate a lot of the boilerplate code involved in handling startup / shutdown of subcomponents into a [single base struct](https://github.com/onflow/flow-go/pull/1079#discussion_r684390781), which could implement all the logic of `Run`, `Ready`, and `Done`. 

Modules which wanted to provide these interfaces would just need to populate a slice field on the base struct with all of its submodules and child go-routines, and the base struct implementation would handle the rest.

What we really need is, a way to specify that certain channels require valid [peer.ID](http://peer.ID) (that belongs to the protocol state), and for those channels we just add a topic validator that verifies the peer.ID by looking it up in the protocol state. We ignore the given originID completely

For other channels, we don't need it at all, and so we just give it the peer.ID

Add the (double network) as a motivation for this change (ie under "Problems with current design").  Also, are we really sure that in the future there will be no need for engines to send messages to arbitrary unstaked peer? What about full nodes, for example?

We can check origin id's against peer IDs, but instead of wasting that compuitation why don't we omit the field entirely

- While engine.unit is nice, it uses antipattern of storing contexts (here is an example that illustrates why it is bad)
#### API Proposal
*  Generalize the API for the message sink from the viewpoint of the networking layer:
   ```golang
   // MessageConsumer represents a consumer of messages for a particular channel from 
   // the viewpoint of the networking layer. MessageConsumer implementations must handle 
   // large message volumes and consume messages in a non-blocking manner. 
   type MessageConsumer interface {
       // Consume submits the given event from the node with the given origin ID
	   // to the MessageConsumer. Implementation must be non-blocking and 
       // internally queue (or drop) messages.   
       Consume(originID [32]byte, event interface{})
    }
   ```
   Note that the `MessageConsumer` has no error return as opposed to the
   Engine's [`Process` method](https://github.com/onflow/flow-go/blob/782c6d4c45007406fc708be22dcfaf9859a62991/network/engine.go#L27).
   Motivation
    * What is the networking layer supposed to do with an error? Currently, the networking layer just logs an error
      (-> [code](https://github.com/onflow/flow-go/blob/9be5cca2697e78f4b2d6c06210d4ac7a4bbfdb64/network/p2p/network.go#L445-L452)).
    * Best suited to handle occurring errors is the application layer (i.e. the `MessageConsumer`), which has the necessary
      context about message semantics and potential errors.
*  change the `Network`'s [`Register` method](https://github.com/onflow/flow-go/blob/782c6d4c45007406fc708be22dcfaf9859a62991/module/network.go#L19) to
   ```golang
   Register(channel network.Channel, consumer MessageConsumer) (network.Conduit, error)
   ```
## Implementation Steps

The goal of the proposed implementation steps is to transition to the new API in iterative steps.
There are numerous `Engine` implementations. For some, it might be a good opportunity 
to implement message queuing tailored to the specific message type(s), which the Engine processes. 
For other Engines, a generic queueing implementation might be sufficient.  

1. In the first step, we will only change the interface, but the implementation will remain functionally unchanged.
   Specifically, we:
     - Add the `MessageConsumer interface` to `network` package.   
     - Move the `Engine interface` out of the networking layer into the `module` package.
     - Keep the priority queue in the networking layer and the workers feeding the messages to the engines.
     - To each engine, we add the `Consume` method, which directly calls into the engine's `Process` method.
       To only other function of `Consume` would be to logg potential error returns from `Process`
   
   This first step would create technical debt, as the `MessageConsumer` implementations are _not conforming to
   the API specification_: because they block on `Consume`!
2. We implement a generic priority queue (maybe reusing the existing implementation from the networking layer).
   - we could consider adding a generic inbound queue implementation to `engine/Unit`, as most engines have a `Unit` instance
3. Now, each `Engine` can be _independently_ refactored to include its own inbound message queue(s), 
   drop messages, or whatever fits best for their specific message type. 
4. After all engines are compliant with the `MessageConsumer interface` (i.e. their `Consume` implementation is non-blocking),
   we can remove the priority queue from the networking layer 

#### Implementation Step (1): minimally invasive transition to `MessageConsumer` interface
The implementation will remain functionally unchanged. The tech debt is cleaned up in subsequent steps 

1. Add the `MessageConsumer interface` to `network` package.
2. Move the `Engine interface` out of the networking layer into the `module` package.   
2. To each `Engine` implementation, add the following `Consume` method
    ```golang
    // MessageConsumer represents a consumer of messages for a particular channel from
    // the viewpoint of the networking layer. MessageConsumer implementations must
    // handle large message volumes and consume messages in a non-blocking manner.
    //
    // Consume is called by networking layer to hand a message from (the node with
    // the given origin ID) to the engine. It conforms to the MessageConsumer 
    // interface. However, the current implementation violates the
    // MessageConsumer's API spec as it uses the networking layer's routine to
    // process the inbound message.
    // TODO: change implementation to non-blocking
    func (e *Engine) Consume(originID flow.Identifier, event interface{}) {
        err := e.Process(originID, event)
        if err != nil {
            e.log.Error().
                Err(err).
                Hex("sender_id", logging.ID(originID)).
                Msg("failed to process inbound message")
        }
    }
    ```
    Note: The `MessageConsumer` implementations are _not conforming to
    the API specification_: because they block on `Consume`!
    While the tech debt created here is comparatively small, it allows 
    us to decouple and potentially parallelize updating the different engine implementations. 

This would be a single issue.

#### Implementation Step (2): add priority queue  
Implementation of a generic priority queue (maybe reusing the existing implementation from the networking layer). 
We could consider adding a generic inbound queue implementation to `engine/Unit`, as most engines have a `Unit` instance.

This would be a single issue and could go on in parallel to step (1)

#### Implementation Step (3): update engine implementations to non-blocking `Consume`  
Each `Engine` is refactored to include its own inbound message queue(s),
drop messages, or whatever fits best for their specific message type. 


For each individual `Engine` implementation, we have a dedicated issue:
  * determine which message processing strategy (queue, drop, etc) fits the engine's use case
  * implementation 
This step is blocked until step (1) and (2) are completed. 
All issues from this step could theoretically be worked on in parallel. 

By completing step (3), all `Engine` implementations are compliant with the `MessageConsumer interface`
(i.e. their `Consume` implementation is non-blocking).


#### Implementation Step (4): remove deprecated logic 
We remove the priority queue from the networking layer. 

This would be a single issue.


#### Implementation Step (5): buffer
Some buffer for technical complications and additional cleanup work.  

#### Optional 
We could remove the `network.Engine` interface and the `SubmitLocal`, `Process` functions that are copied in every engine
to satisfy it. These are rarely used and only exist because `network.Engine` was originally created with more methods
than it needed. 

The needed implementations of `SubmitLocal` or `Process` methods could be renamed with context-specific names
(e.g. `engine.HandleBlock(block)`)

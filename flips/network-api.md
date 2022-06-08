# Network Layer API (Core Protocol)

| Status        | Proposed                                                  |
:-------------- |:--------------------------------------------------------- |
| **FLIP #**    | [1306](https://github.com/onflow/flow-go/pull/1306) |
| **Author(s)** | Simon Zhu (simon.zhu@dapperlabs.com)                      |
| **Sponsor**   | Simon Zhu (simon.zhu@dapperlabs.com)                      |
| **Updated**   | 9/16/2021                                                 |

## Objective

Refactor the networking layer to split it into separate APIs for the public and private network, allow us to implement a strict separation in the code between these two networks.

Enable registering a custom message ID function for the gossip layer.

## Current Implementation

When the network layer receives a message, it will pass the message to the [`Engine`](https://github.com/onflow/flow-go/blob/7763000ba5724bb03f522380e513b784b4597d46/network/engine.go) registered on
the corresponding channel by [calling the engine's `Process` method](https://github.com/onflow/flow-go/blob/d31fd63eb651ed9faf0f677e9934baef6c4d9792/network/p2p/network.go#L406), passing it the Flow ID of the message sender.

[`Multicast`](https://github.com/onflow/flow-go/blob/4ddc17d1bee25c2ab12ceabcf814b702980fdebe/network/conduit.go#L82) is implemented by including a [`TargetIDs`](https://github.com/onflow/flow-go/blob/4ddc17d1bee25c2ab12ceabcf814b702980fdebe/network/message/message.proto#L12) field inside the message, which is published to a specific topic on the underlying gossip network. Upon receiving a new message on the gossip network, nodes must first [validate](https://github.com/onflow/flow-go/blob/4ddc17d1bee25c2ab12ceabcf814b702980fdebe/network/validator/targetValiator.go) that they are one of the intended recipients of the message before processing it.

### Potential problems 

The current network layer API was designed with the assumption that all messages sent and received either target or originate from staked Flow nodes. This is why an engine's [`Process`](https://github.com/onflow/flow-go/blob/master/network/engine.go#L28) method accepts a Flow ID identifying the message sender, and outgoing messages [must specify Flow ID(s)](https://github.com/onflow/flow-go/blob/master/network/conduit.go#L62) as targets.

This assumption is no longer true today. The access node, for example, may communicate with multiple (unstaked) consensus followers. It's perceivable that in the future there will be even more cases where communication with unstaked parties may happen (for example, execution nodes talking to DPS).

Currently, a [`Message`](https://github.com/onflow/flow-go/blob/698c77460bc33d1a8ee8a154f7fe4877bc518a02/network/message/message.proto) which is sent over the network contains many unnecessary fields which can be deduced by the receiver of the message. The only exceptions to this are the `Payload` field (which contains the actual message data) and the `TargetIDs` field (which is used by `Multicast`).

However, all of the existing calls to `Multicast` only target a very small number of recipients (3 to be exact), which means that there is a lot of noise on the network causing nodes to waste CPU cycles processing messages only to ignore them once they realize they are not one of the intended recipients.

## Proposal

We should split the existing network layer API into two distinct APIs / packages for the public and private network, and the `Engine` API should be modified so that the [`Process`](https://github.com/onflow/flow-go/blob/master/network/engine.go#L28) and [`Submit`](https://github.com/onflow/flow-go/blob/master/network/engine.go#L20) methods receive a `Context` as the first argument:

* Private network
  ```golang
  type Engine interface {
    Submit(ctx context.Context, channel Channel, originID flow.Identifier, event interface{})
    Process(ctx context.Context, channel Channel, originID flow.Identifier, event interface{}) error
  }

  type Conduit interface {
    Publish(event interface{}, targetIDs ...flow.Identifier) error
    Unicast(event interface{}, targetID flow.Identifier) error
    Multicast(event interface{}, num uint, targetIDs ...flow.Identifier) error
  }
  ```
* Public network
  ```golang
  type Engine interface {
    Submit(ctx context.Context, channel Channel, senderPeerID peer.ID, event interface{})
    Process(ctx context.Context, channel Channel, senderPeerID peer.ID, event interface{}) error
  }

  type Conduit interface {
    Publish(event interface{}, targetIDs ...peer.ID) error
    Unicast(event interface{}, targetID peer.ID) error
    Multicast(event interface{}, num uint, targetIDs ...peer.ID) error
  }
  ```

Various types of request-scoped data may be included in the `Context` as [values](https://pkg.go.dev/context#WithValue). For example, if a message sent on the public network originates from a staked node, that node's Flow ID may be included as a value. Once engine-side message queues are standardized as described in [FLIP 343](https://github.com/onflow/flow/pull/343), the given `Context` can be placed in the message queue along with the message itself in a wrapper struct:

```golang
type Message struct {
  ctx context.Context
  event interface{}
}
```

> While this may seem to break the general rule of not storing `Context`s in structs, storing `Context`s in structs which are being passed like parameters is one of the exceptions to this rule. See [this](https://github.com/golang/go/issues/22602#:~:text=While%20we%27ve%20told,documentation%20and%20examples.) and [this](https://medium.com/@cep21/how-to-correctly-use-context-context-in-go-1-7-8f2c0fafdf39#:~:text=The%20one%20exception%20to%20not%20storing%20a%20context%20is%20when%20you%20need%20to%20put%20it%20in%20a%20struct%20that%20is%20used%20purely%20as%20a%20message%20that%20is%20passed%20across%20a%20channel.%20This%20is%20shown%20in%20the%20example%20below.). The idea is that `Context`s should not be **stored** but should **flow** through the program, which is what they do in this usecase.

When the message is dequeued, the engine should check the `Context` to see whether the message might already be obsolete before processing it. At this point, we will have two distinct `Context`s in scope:
* The message `Context`
* The `Context` of the goroutine which is dequeing / processing the message

These can be combined into a [single context](https://github.com/teivah/onecontext) which can be used by the message processing business logic, so that the processing can be cancelled either by the network or by the engine. This will allow us to deprecate [`engine.Unit`](https://github.com/onflow/flow-go/blob/master/engine/unit.go), which uses a single `Context` for the entire engine.

There are certain types of messages (e.g block proposals) which may transit between the private and public networks via relay nodes (e.g Access Nodes). Libp2p's [default message ID function](https://github.com/libp2p/go-libp2p-pubsub/blob/0c7092d1f50091ae88407ba93103ac5868da3d0a/pubsub.go#L1040-L1043) will treat a message originating from one network, relayed to the other network by `n` distinct relay nodes, as `n` distinct messages, causing unnacceptable message duplification / traffic amplification. In order to prevent this, we will need to define a [custom message ID function](https://pkg.go.dev/github.com/libp2p/go-libp2p-pubsub#WithMessageIdFn) which returns the hash of the message [`Payload`](https://github.com/onflow/flow-go/blob/698c77460bc33d1a8ee8a154f7fe4877bc518a02/network/message/message.proto#L13). 

In order to avoid making the message ID function deserialize the `Message` to access the `Payload`, we need to remove all other fields from the `Message` protobuf so that the message ID function can simply take the hash of the the pubsub [`Data`](https://github.com/libp2p/go-libp2p-pubsub/blob/0c7092d1f50091ae88407ba93103ac5868da3d0a/pb/rpc.pb.go#L145) field without needing to do any deserialization.

The `Multicast` implementation will need to be changed to make direct connections to the target peers instead of sending messages with a `TargetIDs` field via gossip.

### Motivations
- Having a strict separation between the public and private networks provides better safety by preventing unintended passage of messages between the two networks, and makes it easier to implement mechanisms for message prioritization / rate-limiting on staked nodes which participate in both.
- Passing `Context`s gives the network layer the ability to cancel the processing of a network message. This can be leveraged to implement [timeouts](https://pkg.go.dev/context#WithTimeout), but may also be useful for other situations. For example, if the network layer becomes aware that a certain peer has become unreachable, it can cancel the processing of any sync requests from that peer.
- Since existing calls to `Multicast` only target 3 peers, changing the implementation to use direct connections instead of gossip will reduce traffic on the network and make it more efficient.
- While `engine.Unit` provides some useful functionalities, it also uses the anti-pattern of [storing a `Context` inside a struct](https://github.com/onflow/flow-go/blob/b50f0ffe054103a82e4aa9e0c9e4610c2cbf2cc9/engine/unit.go#L117), something which is [specifically advised against](https://pkg.go.dev/context#:~:text=Do%20not%20store%20Contexts%20inside%20a%20struct%20type%3B%20instead%2C%20pass%20a%20Context%20explicitly%20to%20each%20function%20that%20needs%20it.%20The%20Context%20should%20be%20the%20first%20parameter%2C%20typically%20named%20ctx%3A) by [the developers of Go](https://go.dev/blog/context-and-structs#TOC_2.). Here is an [example](https://go.dev/blog/context-and-structs#:~:text=Storing%20context%20in%20structs%20leads%20to%20confusion) illustrating some of the problems with this approach.

## Implementation (TODO)
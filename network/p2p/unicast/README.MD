# Unicast Manager

## Overview
In Flow blockchain, nodes communicate with each other in 3 different ways; `unicast`, `multicast`, and `publish`.
The `multicast` and `publish` are handled by the pubsub (GossipSub) protocol.
The `unicast` is a protocol that is used to send messages over direct (one-to-one) connections to remote nodes.
Each `unicast` message is sent through a single-used, one-time stream. One can see a stream as a virtual protocol
that expands the base direct connection into a full-duplex communication channel.
Figure below illustrates the notion of direct connection and streams between nodes A and B. The direct 
connection is established between the nodes and then the nodes can open multiple streams over the connection.
The streams are shown with dashed green lines, while the direct connection is illustrated by blue lines that
encapsulates the streams.
![streams.png](streams.png)

The `unicast` `Manager` is responsible for _establishing_ streams between nodes when they need to communicate
over `unicast` protocol. When the manager receives a `CreateStream` invocation, it will try to establish a stream to the
remote `peer` whose identifier is provided in the invocation (`peer.ID`). The manager is expanding the libp2p
functionalities, hence, it operates on the notion of the `peer` (rather than Flow node), and `peer.ID` rather
than `flow.Identifier`. It is the responsibility of the caller to provide the correct `peer.ID` of the remote
node. 

The `UnicastManager` relies on the underlying libp2p node to establish the connection to the remote peer. Once the underlying
libp2p node receives a stream creation request from the `UnicastManager`, it will try to establish a connection to the remote peer if 
there is no existing connection to the peer. Otherwise, it will pick and re-use the best existing connection to the remote peer.
Hence, the `UnicastManager` does not (and should not) care about the connection establishment, and rather relies on the underlying
libp2p node to establish the connection. The `UnicastManager` only cares about the stream creation, and will return an error
if the underlying libp2p node fails to establish a connection to the remote peer.


A stream is a one-time communication channel, i.e., it is assumed to be closed 
by the caller once the message is sent. The caller (i.e., the Flow node) does not necessarily re-use a stream, and the 
`Manager` creates one stream per request (i.e., `CreateStream` invocation), which is typically a single message.

Note: the limit of number of streams and connections between nodes is set throught eh libp2p resource manager limits (see `config/default-config.yml`):

Note: `pubsub` protocol also establishes connections between nodes to exchange gossip messages with each other.
The connection type is the same between `pubsub` and `unicast` protocols, as they both consult the underlying LibP2P node to
establish the connection. However, the level of reliability, life-cycle, and other aspects of the connections are different
between the two protocols. For example, `pubsub` requires some _number_ of connections to some _number_ of peers, which in most cases
is regardless of their identity. However, `unicast` requires a connection to a specific peer, and the connection is assumed
to be persistent. Hence, both these protocols have their own notion of connection management; the `unicast` `Manager` is responsible
for establishing connections when `unicast` protocol needs to send a message to a remote peer, while the `PeerManager` is responsible 
for establishing connections when `pubsub`. These two work in isolation and independent of each other to satisfy different requirements.

The `PeerManager` regularly checks the health of the connections and closes the connections to the peers that are not part of the Flow 
protocol state. One the other hand, the `unicast` `Manager` only establishes a connection if there is no existing connection to the remote
peer. Currently, Flow nodes operate on a full mesh topology, meaning that every node is connected to every other node through `PeerManager`.
The `PeerManager` starts connecting to every remote node of the Flow protocol upon startup, and then maintains the connections unless the node
is disallow-listed or ejected by the protocol state. Accordingly, it is a rare event that a node does not have a connection to another node.
Also, that is the reason behind the `unicast` `Manager` not closing the connection after the stream is closed. The `unicast` `Manager` assumes
that the connection is persistent and will be kept open by the `PeerManager`. 

## Backoff and Retry Attempts
The flowchart below explains the abstract logic of the `UnicastManager` when it receives a `CreateStream` invocation.
On the happy path, the `UnicastManager` successfully opens a stream to the peer.
However, there can be cases that the remote peer is not reliable for stream creation, or the remote peer acts
maliciously and does not respond stream creation requests. In order to distinguish between the cases that the remote peer
is not reliable and the cases that the remote peer is malicious, the `UnicastManager` uses a backoff and retry mechanism.

![retry.png](retry.png)

### Addressing Unreliable Remote Peer
To address the unreliability of remote peer, upon an unsuccessful attempt to establish a stream, the `UnicastManager` will wait for a certain 
amount of time before it tries to establish (i.e., the backoff mechanism), and will retry a certain number of times before it gives up (i.e., the retry mechanism). 
The backoff and retry parameters are configurable through runtime flags.
If all backoff and retry attempts fail, the `UnicastManager` will return an error to the caller. The caller can then decide to retry the request or not.
By default, `UnicastManager` retries each stream creation attempt 3 times. Also, the backoff intervals for dialing and stream creation are initialized to 1 second and progress 
exponentially with a factor of 2, i.e., the `i-th` retry attempt is made after `t * 2^(i-1)`, where `t` is the backoff interval. 
For example, if the backoff interval is 1s, the first attempt is made right-away, the first (retry) attempt is made after 1s * 2^(1 - 1) = 1s, the third (retry) attempt is made 
after `1s * 2^(2 - 1) = 2s`, and so on.

These parameters are configured using the `config/default-config.yml` file:
```yaml
  # Unicast create stream retry delay is initial delay used in the exponential backoff for create stream retries
  unicast-create-stream-retry-delay: 1s
```

### Addressing Malicious Remote Peer
The backoff and retry mechanism is used to address the cases that the remote peer is not reliable. 
However, there can be cases that the remote peer is malicious and does not respond to stream creation requests.
Such cases may cause the `UnicastManager` to wait for a long time before it gives up, resulting in a resource exhaustion and slow-down of the stream creation.
To mitigate such cases, the `UnicastManager` uses a retry budget for the stream creation. The retry budgets are initialized 
using the `config/default-config.yml` file:
```yaml
  # The maximum number of retry attempts for creating a unicast stream to a remote peer before giving up. If it is set to 3 for example, it means that if a peer fails to create
  # retry a unicast stream to a remote peer 3 times, the peer will give up and will not retry creating a unicast stream to that remote peer.
  # When it is set to zero it means that the peer will not retry creating a unicast stream to a remote peer if it fails.
  unicast-max-stream-creation-retry-attempt-times: 3
```

As shown in the above snippet, the stream creation is set to 3 by default for every remote peer.
Each time the `UnicastManager` is invoked on `CreateStream` to `pid` (`peer.ID`), it loads the retry budgets for `pid` from the unicast config cache.
If no unicast config record exists for `pid`, one is created with the default retry budgets. The `UnicastManager` then uses the retry budgets to decide
whether to retry the stream creation attempt or not. If the retry budget for stream creation is exhausted, the `UnicastManager`
will not retry the stream creation attempt, and returns an error to the caller. The caller can then decide to retry the request or not.
Note that even when the retry budget is exhausted, the `UnicastManager` will try the stream creation attempt once, though it will not retry the attempt if it fails.

#### Penalizing Malicious Remote Peer
Each time the `UnicastManager` fails to create a stream to a remote peer and exhausts the retry budget, it penalizes the remote peer as follows:
- If the `UnicastManager` exhausts the retry budget for stream creation, it will decrement the stream creation retry budget for the remote peer.
- If the retry budget reaches zero, the `UnicastManager` will only attempt once to create a stream to the remote peer, and will not retry the attempt, and rather return an error to the caller.
- When the budget reaches zero, the `UnicastManager` will not decrement the budget anymore.

**Note:** `UnicastManager` is part of the networking layer of the Flow node, which is a lower-order component than
the Flow protocol engines who call the `UnicastManager` to send messages to remote peers. Hence, the `UnicastManager` _must not_ outsmart
the Flow protocol engines on deciding whether to _create stream_ in the first place. This means that `UnicastManager` will attempt 
to create stream even to peers with zero retry budgets. However, `UnicastManager` does not retry attempts for the peers with zero budgets, and rather
returns an error immediately upon a failure. This is the responsibility of the Flow protocol engines to decide whether
to send a message to a remote peer or not after a certain number of failures. 

#### Restoring Retry Budgets

The `UnicastManager` may reset the stream creation budget for a remote peers _from zero to the default values_ in the following cases:

- **Restoring Stream Creation Retry Budget**: To restore the stream creation budget from zero to the default value, the `UnicastManager` keeps track of the _consecutive_
  successful streams created to the remote peer. Everytime a stream is created successfully, the `UnicastManager` increments a counter for the remote peer. The counter is
  reset to zero upon the _first failure_ to create a stream to the remote peer. If the counter reaches a certain threshold, the `UnicastManager` will reset the stream creation
  budget for the remote peer to the default value. The threshold is configurable through the `config/default-config.yml` file:
    ```yaml
  # The minimum number of consecutive successful streams to reset the unicast stream creation retry budget from zero to the maximum default. If it is set to 100 for example, it
  # means that if a peer has 100 consecutive successful streams to the remote peer, and the remote peer has a zero stream creation budget,
  # the unicast stream creation retry budget for that remote peer will be reset to the maximum default.
  unicast-stream-zero-retry-reset-threshold: 100
    ```
  Reaching the threshold means that the remote peer is reliable enough to regain the default retry budget for stream creation.
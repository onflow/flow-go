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

If there is existing connection between the local node and the remote node, the manager will try to establish
a connection first, and then open a stream over the connection. The connection is assumed persistent, i.e., it
will be kept until certain events such as Flow node shutdown, restart, disallow-listing of either ends of the connection
by each other, etc. However, a stream is a one-time communication channel, i.e., it is assumed to be closed 
by the caller once the message is sent. The caller (i.e., the Flow node) does not necessarily re-use a stream, and the 
`Manager` creates one stream per request (i.e., `CreateStream` invocation), which is typically a single message.
However, we have certain safeguards in place to prevent nodes from establishing more than one connection to each other.
That is why the `Manager` establishes the connection only when there is no existing connection between the nodes, and otherwise
re-uses the existing connection.

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

## Retry Attempts
![retry.png](retry.png)
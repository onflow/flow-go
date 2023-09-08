# Message Forensic (MF) System

## Summary
This FLIP discusses and compares two potential solutions for the Message Forensic (MF) system in the Flow protocol 
— a system that enables Flow protocol to attribute protocol violations to the original malicious sender. 
The two solutions under consideration are: (1) GossipSub Message Forensic (GMF), and (2) Enforced Flow-level Signing Policy For All Messages. 
We delve into both, listing their pros and cons, to determine which would be more feasible given the considerations of ease of implementation, 
performance efficiency, and security guarantees.

Our analysis finds the "Enforced Flow-level Signing Policy For All Messages" to be the more promising option, 
offering a generalized solution that doesn’t hinge on a specific external protocol to send the message (e.g., GossipSub),
steering clear of the complexities tied to maintaining GossipSub envelopes and dodging the necessity of duplicating GossipSub 
router’s signature verification procedure at the engine level. Furthermore, it meshes well with the Flow protocol’s existing state.

## Problem Overview
Within the Flow protocol, nodes converse through a networking layer, a medium which undertakes the dispatch and receipt of messages among nodes
via different communication methods: unicast, multicast, and publish. Unicast messages are transmitted to a single recipient over a direct connection.
The multicast and publish, on the other hand, utilize a pub-sub network constructed with the LibP2P GossipSub protocol for message 
dissemination. However, this protocol encounters challenges in message attribution, particularly in determining the 
initial sender of a message, which is critical for identifying protocol violations and penalizing the malicious initiators. 
Below, we break down the issues seen in different communication strategies and their implications.

### Single Recipient (Unicast) Message Attribution
Unicast messages have an implicit attribution to the sender, i.e., they are attributed to the remote node of the connection
on which the message is received. Albeit, the situation differs when the unicast message carries the explicit signature of the
sender or other attribution data (e.g., the signature of the original sender in case of a forwarded unicast).
In such cases, the message is attributed to the sender that is specified in the attribution data. Nevertheless, the current state of
Flow protocol does not enforce the attribution data to be present in the unicast messages. 
A prime example is the `ChunkDataPackResponse` messages sent by the Execution Nodes to the Verification Nodes over a unicast. 
Hence a node in Flow protocol cannot prove that a received unicast message is sent by a specific node, unless the message carries the explicit attribution data.

### Group and All-Node (Multicast and Publish) Message Attribution
When dealing with multicast or publish messages, the GossipSub router initially signs it with a local node's networking key 
before releasing it into the pub-sub network. The routers are structured to verify message signatures before forwarding them or making 
them accessible to the application layer (i.e., Flow protocol engines). This method does indeed block invalid messages from progressing through the network, 
also penalizing the immediate sender of an invalid message through a built-in scoring mechanism that influences network connections and message forwarding decisions.
Yet, a significant loophole remains: the elimination of authentication data (like signatures) by the GossipSub router during message delivery to the application layer. 
This eradication obstructs the Flow protocol's ability to trace a message back to its original sender in cases of protocol violations that happen at a level above the GossipSub layer.


## Proposal-1: GossipSub Message Forensic (GMF)
As the first option to (partially) mitigate the gap in the network to hold the malicious senders accountable,
we introduce the GossipSub Message Forensic (GMF) mechanism designed to integrate GossipSub authentication data seamlessly with the 
Flow protocol engines. The principal aim is to enhance message authenticity verification through multicast and publish,
focusing on origin identification and event association. Here, we elucidate the method, its interface, and delve into the operational specifics, 
weighing its pros and cons against potential alternatives.
In this proposal, we propose a mechanism to share the GossipSub authentication data with the Flow protocol. We call this mechanism
GossipSub Message Forensic (GMF). The idea is to add a new interface method to the `EngineRegistry` interface. The `EngineRegistry` was
previously known as the `Network` interface, and is exposed to individual Flow protocol engines who are interested in receiving the
messages from the networking layer including the GossipSub protocol. In this approach, the `EngineRegistry` interface is extended
by two new methods: `GetGossipSubMessageForEvent` and `VerifyGossipSubMessage`. The `GetGossipSubMessageForEvent` method takes an origin 
identifier as well as an event as input, and returns the GossipSub envelope of the message that is associated with the event and 
is sent by the origin identifier.
The `VerifyGossipSubMessage` method takes a origin identifier as well as a GossipSub message as input, and verifies the signature of
the message against the networking public key of the origin id. 
The method returns true if the signature is valid, and false otherwise. 
The event is the scrapped message that is delivered to the application layer by the GossipSub router. The GossipSub envelope
contains the signature of the message, as well as all the other GossipSub metadata that is needed to verify the signature. 
```go

type EngineRegistry interface {
	// GetGossipSubMessageForEvent takes a Flow identifier as well as an event as input, and returns the GossipSub envelope of the
	// message that is associated with the event and is sent by the origin identifier. The event is the scrapped message that is delivered to the application layer by
	// the GossipSub router. The GossipSub envelope contains the signature of the message, as well as all the other GossipSub metadata
	// that is needed to verify the signature.
	// Args:
	//   - originId: The Flow identifier of the node that originally sent the message.
	//  - event: The scrapped message that is delivered to the application layer by the GossipSub router.
	// Returns:
	//  - message: The GossipSub envelope of the message that is associated with the event.
	//  - error: An error if the message is not found.
	GetGossipSubMessageForEvent(originId flow.Identifier, event interface{}) (*pb.Message, error)
	
	// VerifyGossipSubMessage takes a Flow identifier as well as a GossipSub message as input, and verifies the signature of the
	// message. The method returns true if the signature is valid, and false otherwise.
	// Args:
	//  - originId: The Flow identifier of the node that originally sent the message.
	// - message: The GossipSub message that is associated with the event.
	// Returns:
	//  - bool: True if the signature is valid, and false otherwise.
	//  - error: An error if the message is not found.
	VerifyGossipSubMessage(originId flow.Identifier, message *pb.Message) (bool, error)
	
	// Other methods are omitted for brevity
	// ...
}
```

### Implementation Complexities
- This proposal requires the `LibP2PNode` implementation extract the individual envelopes (i.e., `*pb.Message`) from the incoming GossipSub RPCs. 
  This is done using the RPC inspector dependency injection. We need to implement a new inspector that extract the envelopes in a non-blocking approach. 
  We should not block an incoming RPC to extract its messages, as it is interfering with the GossipSub router operation and can harm the message delivery rate of the router. 
- The extracted envelopes are then required to stored persistently by their event id in a forensic component with proper ejection mechanism, 
  which is exposed to the `GetGossipSubMessageForEvent` to query the envelope corresponding to a `(event, originId)` pair. 
- This solution also requires replicating the signature verification logic of the GossipSub in `VerifyGossipSubMessage` so that it is accessible to the engines. 
  We need to extend the signature verification mechanism to account for translation of `originId` from `flow.Identifier` to networking key and `peer.ID` (i.e., LibP2P level identifier).
  As the engines are operating based on the `flow.Identifier`, while the GossipSub signatures are generated using the Networking Key of the node.

### Advantages
1. The GossipSub authentication data is shared with the Flow protocol, providing attribution for `Multicast` and `Publish` messages.
2. The interface is easy to use for the engines, as it abstracts the complexity of translating the origin Flow identifier to the GossipSub peer id, and
   verifying the signature of the message against the networking public key of the origin id.
3. The implementation is not a breaking change and is backward compatible with the current state of the Flow protocol.

### Disadvantages
1. The GossipSub envelope is extractable through an RPC inspector dependency injection, which must be non-blocking and fast (a principle condition
   imposed by GossipSub). This means that the GossipSub envelope extraction must be done in a separate goroutine _asynchronously_ to the message delivery to the
   application layer. This entails that there can be a glitch between the message delivery to the application layer and the GossipSub envelope extraction. The potential glitch
   means that the GossipSub envelope may not be available at the time of the message delivery to the application layer. Accounting for the glitch implies higher complexity in
   the implementation of the Flow protocol engines (e.g., timeout queries, etc.). Moreover, the glitch may be exploited by the malicious nodes to perform timing
   attacks on the Flow protocol engines and get away from detection. If an attacker can time the message delivery to the application layer in a way that the 
   GossipSub envelope is not available at the time of the message delivery, then the attacker can send an invalid message and get away from detection, as when
   there is no GossipSub envelope available, the Flow protocol engines cannot have forensic evidence to attribute the protocol-level violation to the malicious
   sender.
2. Maintaining the GossipSub envelopes poses several challenges:
   - There must be an eviction policy to remove the GossipSub envelopes from the memory (or disk) after a certain period of time. Lack of an eviction policy may
     lead to memory (or disk) exhaustion attacks.
   - Existence of eviction policy may lead to data loss, as the GossipSub envelope may be evicted from the memory (or disk) before the Flow protocol engines
     can use it to attribute a protocol-level violation to the malicious sender.
   - Existence of eviction policy may also lead to attacks; if an attacker can time the message delivery to the application layer in a way that the GossipSub
     envelope is evicted from the memory (or disk) before the Flow protocol engines can use it to attribute a protocol-level violation to the malicious sender,
     then the attacker can send an invalid message and get away from detection, as when there is no GossipSub envelope available, the Flow protocol engines cannot
     have forensic evidence to attribute the protocol-level violation to the malicious sender.
   - On a happy path when all nodes are honest, the GossipSub envelopes are not needed, and extracting and maintaining them poses a performance overhead. 
3. This solution requires replicating the signature verification logic of the GossipSub router at the engine level (i.e., `VerifyGossipSubMessage`). Changes to the GossipSub signature 
    verification procedure may pose as breaking changes and be a blocker for upgrading and keeping up with GossipSub upgrades.

## Proposal-2: Enforced Flow-level Signing Policy For All Messages
In this proposal, we propose to enforce a Flow-level signing policy for all messages. The idea is to refactor the `Conduit` interface, so that instead of 
taking an `interface{}` type event, it takes an `Envelope` type event. The `Envelope` type event is a wrapper around the `interface{}` type event, and contains
the Staking Key signature of the sender on the event. The `Envelope` type is defined as follows:
```go
type Envelope struct {
    // The event that is wrapped by the envelope.
    Event interface{}
    // The Staking Key signature of the sender on the event.
    Signature []byte
}
```

Accordingly, the `Conduit` interface is refactored as follows:
```go
type Conduit interface {
	Publish(envelope *Envelope, targetIDs ...flow.Identifier) error
	
	Unicast(envelope *Envelope, targetID flow.Identifier) error
	
	Multicast(event *Envelope, num uint, targetIDs ...flow.Identifier) error
	
	// Other methods are omitted for brevity
	// ...
}
```

In this design, as a safety mechanism, the engines are required to sign all the messages that are sent through the `Conduit` interface. The `Conduit` interface 
rejects any message that is not signed by the sender. On the other hand, the `Engine` (i.e., `MessageProcessor`) interface is refactored to receive an `Envelope` 
type event instead of an `interface{}` type event. The `Engine` interface is refactored as follows:
```go
// MessageProcessor represents a component which receives messages from the
// networking layer. Since these messages come from other nodes, which may
// be Byzantine, implementations must expect and handle arbitrary message inputs
// (including invalid message types, malformed messages, etc.). Because of this,
// node-internal messages should NEVER be submitted to a component using Process.
type MessageProcessor interface {
	// Process is exposed by engines to accept messages from the networking layer.
	// Implementations of Process should be non-blocking. In general, Process should
	// only queue the message internally by the engine for later async processing.
	Process(channel channels.Channel, originID flow.Identifier, envelope *Envelope) error
}
```
Prior to delivering the message to the engine (i.e., `MessageProcessor`), the networking layer will verify the signature of the Envelope against the Staking Key of the
sender. If the signature is valid, the message will be delivered to the engine. Otherwise, the message will be rejected and reported to the Application Layer Spam Prevention (ALSP system).
In this way, when an engine's `Process` method is called, the engine can be sure that the message is signed by the sender. Hence, the engine can attribute a protocol-level violation to the malicious sender that originally
sent the message.

### Advantages
1. The Flow protocol can attribute a protocol-level violation to the malicious sender that originally sent the message.
2. The implementation is simple; contrary to the GMF proposal, there is no need to extract and maintain the GossipSub envelopes, no extra processing and memory overhead is
   needed.
3. The signature and event are encapsulated together and are passed to the engine as a single object. Hence, the engine does not need to worry about the signature and event
   being out of sync. The lifecycle of the signature and event are managed by the engine itself, than dividing the responsibility between the engine and the networking layer.
4. This solution not only covers the GossipSub messages, but also covers all the messages that are sent through the `Conduit` interface. Hence, it is a more general solution
   that can be used to attribute a protocol-level violation to the malicious sender that originally sent the message, regardless of the protocol that is used to send the message.
   For example, the current state of codebase does not enforce signature policy for the `ChunkDataPack` responses that Execution Nodes send to the Verification Nodes over unicast. Hence,
   a verification node does not have any forensic evidence to attribute a protocol-level violation to the malicious Execution Node that originally sent the message.
5. The entire authentication data is at the Flow protocol level (i.e., Staking Key) and is not dependent on the GossipSub protocol. Hence, the Flow protocol can attribute a
   protocol-level violation to the malicious sender that originally sent the message, regardless of the protocol that is used to send the message.
6. On the long term outlook, we may omit the signature field from individual `Entity` structures of Flow codebase, and instead rely on the `Envelope` signature field. This is a potential optimization that
    we may need to research further. 

### Disadvantages
1. The implementation is a breaking change and is not backward compatible with the current state of the Flow protocol. We change the `Conduit` interface, which is used by
   all the Flow protocol engines to send messages to the networking layer. Hence, all the Flow protocol engines must be refactored to sign the messages that are sent through
   the `Conduit` interface. 
2. This approach adds a computation overhead to the Flow protocol engines, as the engines must sign all the messages that are sent through the `Conduit` interface, and on the receiving side, the 
   the networking layer must verify the signature of the message against the Staking Key of the sender. This overhead is not negligible, as the Flow protocol engines are the most performance critical components of the Flow blockchain. 
   Hence, we must carefully evaluate the performance overhead of this approach. Moreover, with this approach, we are extending the size of data sent over the wire by piggybacking the signature.
   Assuming that we are using ECDSA with `secp521r1` curve and SHA-512 for hashing, the signature size is ~132 bytes (in theory). Hence, we are adding ~132 bytes to the size of data sent over the wire.
   Nevertheless, the performance overhead and the size of data sent over the wire may be an acceptable trade-off for the security guarantees that this approach provides.

### Do we still need GossipSub router signature?
Yes, even when going with the Proposal-2, we still need the GossipSub router signature. This is because the GossipSub router signature is used to verify the authenticity of the message at the GossipSub level.
That is a performant way to filter out the invalid messages at the lower level, and avoid delivering them to the application layer or relaying them to other nodes. One can conceptualize the GossipSub-level signatures
to the ephemeral signatures that are used in secure sessions (e.g., TLS, SSL) to verify the authenticity of the messages at the session level. The ephemeral signatures are used to verify the authenticity of the messages
at the session level, but are not persisted and are not used to attribute a message to the original sender post session. Application context-based signatures are used to attribute a message to the original sender post session.

## References
[1] [Conduit Interface]()

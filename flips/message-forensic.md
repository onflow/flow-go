# Message Forensic (MF) System

## Abstract
In GossipSub the sender of a message signs its published messages.
Upon reception of a new gossiped message, the receiver verifies the signature prior to delivering it to the application
layer or forwarding it to the other nodes (as part of the GossipSub protocol). There are several cases where we need the
GossipSub authentication data to be shared with the Flow protocol as part of the protocol-level decision makings,
e.g., to attribute a protocol-level violation to the malicious sender that originally sent it. The challenge is the signature
is done on the message payload at the GossipSub level, which is not available to the Flow protocol. In this FLIP we propose
a mechanism to share the GossipSub authentication data with the Flow protocol. We call this mechanism GossipSub Message
Forensic (GMF). We also discuss the use cases of GMF in the Flow protocol, its interface, and the implementation details.
Moreover, we articulate the advantages and disadvantages of GMF, and propose a set of alternatives to GMF and compare
them with GMF. The purpose of this FLIP is to provide a fair comparison between all the alternatives and GMF, and to
evaluate the feasibility of the suitable option considering the implementation complexity, performance overhead, and the security
guarantees.

## Problem Definition
GossipSub is a pub-sub protocol that is used in the Flow protocol to disseminate the messages epidemically through the
`Publish` and `Multicast` primitives at conduits [1]. GossipSub router is part of the networking layer of Flow blockchain, that
is responsible for publishing messages to the pubsub network, and relaying the pubsub messages among other GossipSub routers, and
delivering the messages destined to the local Flow node to the application layer (i.e., the Flow protocol engines).
When a GossipSub router receives a publish or multicast message from its application layer, it signs the message with the
networking key of the local node, and then publishes the message to the pubsub network. Upon reception of a new gossiped message,
each GossipSub router verifies the signature of the message prior to delivering it to the application layer or forwarding it to
the other nodes (as part of the GossipSub protocol).

If a node receives a message with an invalid signature, it will not deliver the message to the application layer, and will not
forward the message to the other nodes. Rather, the receiver penalizes the node that has directly sent the message to it. The
penalty is done by applying a misbehavior penalty to the node's score in the local GossipSub scoring mechanism, which is used to
select the peers to forward the messages to as well as to decide whether to accept or reject a new peer connection. Hence, if a
malicious node keeps sending invalid messages to the other nodes, it will be penalized and eventually will be disconnected from
the network. This approach implies the following assumptions valid:
1. When the signature of a message is invalid, the message is not delivered to the application layer. Hence, the application layer
   only receives messages with valid signatures from the _original_ sender.
2. We already built in the mechanism to penalize the malicious nodes that send or relay invalid messages. Hence, we scope out the
   discussion of the malicious nodes that send or relay invalid messages from this FLIP.

The challenge is when the current state of Flow protocol is not capable of attributing a received message through GossipSub to the
original sender. This is because the GossipSub authentication date (i.e., the signature) is scrapped by the GossipSub router when
delivering the message to the application layer. Hence, the Flow protocol cannot attribute a received message to the original sender.
This is a problem because although a message may be valid at the GossipSub level, it may be invalid at the Flow protocol level, e.g.,
an invalid block proposal, an invalid verification result approval, etc. In such cases, the Flow protocol needs to attribute the
protocol-level violation to the malicious sender that originally sent the message.


## Proposal-1: GossipSub Message Forensic (GMF)
In this proposal, we propose a mechanism to share the GossipSub authentication data with the Flow protocol. We call this mechanism
GossipSub Message Forensic (GMF). The idea is to add a new interface method to the `EngineRegistry` interface. The `EngineRegistry` was
previously known as the `Network` interface, and is exposed to individual Flow protocol engines who are interested in receiving the
messages from the networking layer including the GossipSub protocol. In this approach, the `EngineRegistry` interface is extended
by two new methods: `GetGossipSubMessageForEvent` and `VerifyGossipSubMessage`. The `GetGossipSubMessageForEvent` method takes an origin 
identifier as well as an event as input, and returns the GossipSub envelope of the message that is associated with the event and 
is sent by the origin identifier.
The `VerifyGossipSubMessage` method takes a origin identifier as well as a GossipSub message as input, and verifies the signature of
the message against the networking public key of the origin id. 
The method returns true if the signature is valid, and false otherwise. The `GetGossipSubMessageForEvent` method is
The event is the scrapped message that is delivered to the application layer by the GossipSub router. The GossipSub envelope
message. The event is the scrapped message that is delivered to the application layer by the GossipSub router. The GossipSub envelope
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

### Advantages
1. The GossipSub authentication data is shared with the Flow protocol.
2. The interface is easy to use, as it abstracts the complexity of translating the origin Flow identifier to the GossipSub peer id, and
   verifying the signature of the message against the networking public key of the origin id.
3. The implementation is not a breaking change and is backward compatible with the current state of the Flow protocol.

### Disadvantages
1. The GossipSub envelope is extractable through an RPC inspector dependency injection, which must be non-blocking and fast (a principle condition
   imposed by GossipSub). This means that the GossipSub envelope extraction must be done in a separate goroutine _asynchronously_ to the message delivery to the
   application layer. This entails that there can be a glitch between the message delivery to the application layer and the GossipSub envelope extraction, which
   means that the GossipSub envelope may not be available at the time of the message delivery to the application layer. The glitch implies higher complexity in
   the implementation of the Flow protocol engines (e.g., timeout queries, etc.). Moreover, the glitch may be exploited by the malicious nodes to perform timing
   attacks on the Flow protocol engines and get away from detection. If an attacker can time the message delivery to the application layer in a way that the 
   GossipSub envelope is not available at the time of the message delivery, then the attacker can send an invalid message and get away from detection, as when
   there is no GossipSub envelope available, the Flow protocol engines cannot have forensic evidence to attribute the protocol-level violation to the malicious
   sender.
2. 
   

## References
[1] [Conduit Interface]()

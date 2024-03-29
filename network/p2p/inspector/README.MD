# Control Message Validation Inspector Overview

## Component Overview
The Control Message Validation Inspector (`ControlMsgValidationInspector`) is an injectable component responsible for asynchronous inspection of incoming GossipSub RPC.
It is entirely developed and maintained at Flow blockchain codebase and is injected into the GossipSub protocol of libp2p at the startup of the node.
All incoming RPC messages are passed through this inspection to ensure their validity and compliance with the Flow protocol semantics.

The inspector performs two primary functions:
1. **RPC truncation (blocking)**: It truncates size of incoming RPC messages to prevent excessive resource consumption, if needed. This is done by sampling the messages and reducing their size to a configurable threshold.
2. **RPC inspection (aka validation) (non-blocking)**: It inspects (aka validates) the truncated or original RPC messages for compliance with the Flow protocol semantics. This includes validation of message structure, topic, sender, and other relevant attributes.

Figure below shows the high-level overview of the Control Message Validation Inspector and its interaction with the GossipSub protocol and the Flow node.
The blue box represents the GossipSub protocol, which is responsible for handling the pub-sub messaging system and is an external dependency of the Flow node.
The green boxes represent various components of the Flow node's networking layer that are involved in the inspection and processing of incoming RPC messages.
The steps that are marked with an asterisk (*) are performed concurrently, while the rest are performed sequentially.
As shown in this figure, an incoming RPC message is passed by GossipSub to the Control Message Validation Inspector, which then performs the blocking truncation process and queues the RPC for asynchronous non-blocking inspection processes.
As soon as the RPC is queued for inspection, it is also passed to the GossipSub protocol for further processing. The results of the inspection are used for internal metrics, logging, and feedback to the GossipSub scoring system.
Once the GossipSub processes the RPC it passes the message to the libp2p node component of the networking layer of the Flow node, which then processes the message and sends it to the rest of the Flow node for further processing.
Note that the validation process is non-blocking, hence even a malformed RPC is allowed to proceed through the GossipSub protocol to the Flow node. 
However, based on the result of the asynchronous inspection, the message may be scored negatively, and the sender may be penalized in the peer scoring system.
The rationale behind this is that post truncation, as far as the RPC size is within the configured limits, a single (or few) non-compliant RPCs do not drastically affect the system's health, hence, the RPCs are allowed to proceed for further processing.
What matters is the persistent behavior of the sender, and the sender's reputation and future message propagation are _eventually_ affected based on the inspection results.
![rpc-inspection-process.png](rpc-inspection-process.png)
## What is an RPC?
RPC stands for Remote Procedure Call. In the context of GossipSub, it is a message that is sent from one peer to another peer over the GossipSub protocol. 
The message is sent in the form of a protobuf message and is used to communicate information about the state of the network, such as topic membership, message propagation, and other relevant information.
It encapsulates various types of messages and commands that peers exchange to implement the GossipSub protocol, a pub-sub (publish-subscribe) messaging system. 
Remember that the purpose of GossipSub is to efficiently disseminate messages to interested subscribers in the network without requiring a central broker or server.
Here is what an RPC message looks like in the context of GossipSub:
```go
type RPC struct {
	Subscriptions        []*RPC_SubOpts  `protobuf:"bytes,1,rep,name=subscriptions" json:"subscriptions,omitempty"`
	Publish              []*Message      `protobuf:"bytes,2,rep,name=publish" json:"publish,omitempty"`
	Control              *ControlMessage `protobuf:"bytes,3,opt,name=control" json:"control,omitempty"`
	XXX_NoUnkeyedLiteral struct{}        `json:"-"`
	XXX_unrecognized     []byte          `json:"-"`
	XXX_sizecache        int32           `json:"-"`
}
``` 

Here's a breakdown of the components within the GossipSub's `RPC` struct:
1. **Subscriptions (`[]*RPC_SubOpts`)**: This field contains a list of subscription options (`RPC_SubOpts`). 
    Each `RPC_SubOpts` represents a peer's intent to subscribe or unsubscribe from a topic. 
    This allows peers to dynamically adjust their interest in various topics and manage their subscription list.
2. **Publish (`[]*Message`)**: The `Publish` field contains a list of messages that the peer wishes to publish (or gossip) to the network. 
    Each `Message` is intended for a specific topic, and peers subscribing to that topic should receive the message. 
    This field is essential for the dissemination of information and data across the network.
3. **Control (`*ControlMessage`)**
    The `Control` field holds a control message, which contains various types of control information required for the operation of the GossipSub protocol. 
    This can include information about grafting (joining a mesh for a topic), pruning (leaving a mesh), 
    and other control signals related to the maintenance and optimization of the pub-sub network. 
   The control messages play a crucial role in the mesh overlay maintenance, ensuring efficient and reliable message propagation.
4. **XXX Fields** These fields (`XXX_NoUnkeyedLiteral`, `XXX_unrecognized`, and `XXX_sizecache`) are generated by the protobuf compiler and are not directly used by the GossipSub protocol. 
   They are used internally by the protobuf library for various purposes like caching and ensuring correct marshalling and unmarshalling of the protobuf data.

### Closer Look at the Control Message
In GossipSub, a Control Message is a part of the `RPC` structure and plays a crucial role in maintaining and optimizing the network. 
It contains several fields, each corresponding to different types of control information.
The primary purpose of these control messages is to manage the mesh overlay that underpins the GossipSub protocol,
ensuring efficient and reliable message propagation.

At the core, the control messages are used to maintain the mesh overlay for each topic, allowing peers to join and leave the mesh as their interests and network connectivity change.
The control messages include the following types:

1. **IHAVE (`[]*ControlIHave`)**: the `IHAVE` messages are used to advertise to peers that the sender has certain messages. 
   This is part of the message propagation mechanism. 
   When a peer receives an `IHAVE` message and is interested in the advertised messages (because it doesn't have them yet), 
   it can request those messages from the sender using an `IWANT` message.

2. **IWANT (`[]*ControlIWant`)**: the `IWANT` messages are requests sent to peers to ask for specific messages previously 
   advertised in an `IHAVE` message. 
   This mechanism ensures that messages propagate through the network, 
   reaching interested subscribers even if they are not directly connected to the message's original publisher.

3. **GRAFT (`[]*ControlGraft`)**: The `GRAFT` messages are used to express the sender's intention to join the mesh for a specific topic. 
   In GossipSub, each peer maintains a local mesh network for each topic it is interested in. 
   Each local mesh is a subset of the peers in the network that are interested in the same topic. The complete mesh for a topic is formed by the union of all local meshes, which must be connected to ensure efficient message propagation 
   (the peer scoring ensures that the mesh is well-connected and that peers are not overloaded with messages)
   Sending a `GRAFT` message is a way to join the local mesh of a peer, indicating that the sender wants to receive and forward messages for the specific topic.

4. **PRUNE (`[]*ControlPrune`)**: conversely, `PRUNE` messages are sent when a peer wants to leave the local mesh for a specific topic. 
   This could be because the peer is no longer interested in the topic or is optimizing its network connections. 
   Upon receiving a `PRUNE` message, peers will remove the sender from their mesh for the specific topic.

```go
type ControlMessage struct {
	Ihave                []*ControlIHave `protobuf:"bytes,1,rep,name=ihave" json:"ihave,omitempty"`
	Iwant                []*ControlIWant `protobuf:"bytes,2,rep,name=iwant" json:"iwant,omitempty"`
	Graft                []*ControlGraft `protobuf:"bytes,3,rep,name=graft" json:"graft,omitempty"`
	Prune                []*ControlPrune `protobuf:"bytes,4,rep,name=prune" json:"prune,omitempty"`
	XXX_NoUnkeyedLiteral struct{}        `json:"-"`
	XXX_unrecognized     []byte          `json:"-"`
	XXX_sizecache        int32           `json:"-"`
}
```

## Why is RPC Inspection Necessary?
In the context of the Flow blockchain, RPC inspection is necessary for the following reasons:
1. **Security**: The inspection process mitigates potential security risks such as spamming, message replay attacks, or malicious content dissemination, and provides complementing feedbacks for the internal GossipSub scoring system.

2. **Resource Management**: By validating and potentially truncating incoming RPC messages, the system manages its computational and memory resources more effectively. 
   This prevents resource exhaustion attacks where an adversary might attempt to overwhelm the system by sending a large volume of non-compliant or oversized messages.

3. **Metrics and Monitoring**: The inspection process provides valuable insights into the network's health and performance. 
   By monitoring the incoming RPC messages, the system can collect metrics and statistics about message propagation, topic membership, and other relevant network attributes.

## RPC Truncation (Blocking)
The Control Message Validation Inspector is responsible for truncating the size of incoming RPC messages to prevent excessive resource consumption. This is done by sampling the messages and reducing their size to a configurable threshold. 
The truncation process is entirely done in a blocking manner, i.e., it is performed at the entry point of the GossipSub through an injected interceptor, and the incoming RPC messages are modified before they are further processed by the GossipSub protocol.
The truncation process is applied to different components of the RPC message, specifically the control message types (`GRAFT`, `PRUNE`, `IHAVE`, `IWANT`) and their respective message IDs.
Truncation is triggered if the count of messages or message IDs exceeds certain configured thresholds, ensuring that the system resources are not overwhelmed.
When the number of messages or message IDs exceeds the threshold, a random sample of messages or message IDs is selected, and the rest are discarded.

### Message vs Message ID Truncation
In the context of GossipSub RPC inspection, there is a subtle distinction between the count of messages and the count of message IDs:

1. **Count of Messages:**
    - This refers to the number of control messages (like `GRAFT`, `PRUNE`, `IHAVE`, `IWANT`) that are part of the `ControlMessage` structure within an RPC message, i.e., size of the `Graft`, `Prune`, `Ihave`, and `Iwant` slice fields.
    - Each control message type serves a different purpose in the GossipSub protocol (e.g., `GRAFT` for joining a mesh for a topic, `PRUNE` for leaving a mesh).
    - When we talk about the "count of messages," we're referring to how many individual control messages of each type are included in the RPC.
    - Truncation based on the count of messages ensures that the number of control messages of each type doesn't exceed a configured threshold, preventing overwhelming the receiving peer with too many control instructions at once.

2. **Count of Message IDs:**
    - This refers to the number of unique identifiers for actual published messages that are being referenced within control messages like `IHAVE` and `IWANT`.
    - `IHAVE` messages contain IDs of messages that the sender has and is announcing to peers. `IWANT` messages contain IDs of messages that the sender wants from peers.
    - Each _individual_ `IHAVE` or `IWANT` control message can reference multiple message IDs. The "count of message IDs" is the total number of such IDs contained within each `IHAVE` or `IWANT` control message.
    - Truncation based on the count of message IDs ensures that each `IHAVE` or `IWANT` control message doesn't reference an excessively large number of messages. This prevents a scenario where a peer might be asked to process an overwhelming number of message requests at once, which could lead to resource exhaustion.

## RPC Validation (Non-Blocking)
The Control Message Validation Inspector is also responsible for inspecting the truncated or original RPC messages for compliance with the Flow protocol semantics. 
The inspection process is done post truncation and is entirely non-blocking, i.e., it does not prevent the further processing of the RPC messages by the GossipSub protocol. 
In other words, the RPC messages are passed through after truncation for further processing by the GossipSub protocol, regardless of whether they pass the inspection or not.
At the same time, each incoming RPC message is queued for asynchronous inspection, and the results of the inspection are used for internal metrics, logging, and feedback to the GossipSub scoring system.
This means that even a non-compliant RPC message is allowed to proceed through the GossipSub protocol to the Flow node. However, based on the result of the asynchronous inspection, 
the message may be scored negatively, and the sender may be penalized in the peer scoring system. Hence, its future messages may be de-prioritized or ignored by the GossipSub protocol.
This follows the principle that post truncation, as far as the RPC size is within the configured limits, a single (or few) non-compliant RPCs do not drastically affect the system's health,
hence, the RPCs are allowed to proceed for further processing. However, the sender's reputation and future message propagation are affected based on the inspection results.

The queued RPCs are picked by a pool of worker threads, and the inspection is performed in parallel to the GossipSub protocol's processing of the RPC messages.
Each RPC message is inspected for the following attributes sequentially, and once a non-compliance is detected, the inspection process is terminated with a failure result. A failure result
will cause an _invalid control message notification_ (`p2p.InvCtrlMsgNotif`) to be sent to the `GossipSubAppSpecificScoreRegistry`, which will then be used for penalizing the sender in the peer scoring system.
The `GossipSubAppSpecificScoreRegistry` is a Flow-level component that decides on part of the individual peer's scoring based on their Flow-specific behavior. 
It directly provides feedback to the GossipSub protocol for scoring the peers.

The [order of inspections for a single RPC](https://github.com/onflow/flow-go/blob/master/network/p2p/inspector/validation/control_message_validation_inspector.go#L270-L323) is as follows. Note that in the
descriptions below, when we say an RPC is flagged as invalid or the inspection process is terminated with a failure result, and an _invalid control message notification_ is sent to the `GossipSubAppSpecificScoreRegistry`, which
will then be used for penalizing the sender in the peer scoring system.
1. `GRAFT` messages validation: Each RPC contains one or more `GRAFT` messages. Each `GRAFT` message contains a topic ID indicating the mesh the peer wants to join. 
    The validation process involves iterating through each `GRAFT` message received in the (potentially truncated) RPC.
    For each `GRAFT` message, the topic ID is validated to ensure it corresponds to a valid and recognized topic within the Flow-network.
    Topic validation might involve checking if the topic is known, if it's within the scope of the peer's interests or subscriptions, and if it aligns with the network's current configuration (e.g., checking against the active spork ID).
    If the topic is cluster-prefixed, additional validations ensure that the topic is part of the active cluster IDs. 
    If (even one) topic ID is invalid or unrecognized, the `GRAFT` message is flagged as invalid, and the inspection process is terminated with a failure result. 
    In future we may relax this condition to allow for a certain number of invalid topics, but for now, a single invalid topic results in a failure.
    The inspection process also system keeps track of the topics seen in the `GRAFT` messages of the same RPC. 
    If a topic is repeated (i.e., if there are duplicate topics in the `GRAFT` messages of the same RPC), this is usually a sign of a protocol violation or misbehavior.
    The validation process counts these duplicates and, if the number exceeds a certain threshold, it flags RPC message as invalid and terminates the inspection process with a failure result.
    Note that all `GRAFT` messages on the same (potentially truncated) RPC are validated together, without any sampling, as the number of `GRAFT` messages is usually assumed small, and validating
    them is not assumed to be resource-intensive.
2. `PRUNE` messages validation: Similar to `GRAFT`s, each RPC contains one or more `PRUNE` messages. Each `PRUNE` message contains a topic ID indicating the mesh the peer wants to leave. 
    The validation process involves iterating through each `PRUNE` message received in the (potentially truncated) RPC.
    For each `PRUNE` message, the topic ID is validated to ensure it corresponds to a valid and recognized topic within the Flow-network.
    Topic validation might involve checking if the topic is known, if it's within the scope of the peer's interests or subscriptions, and if it aligns with the network's current configuration (e.g., checking against the active spork ID).
    If the topic is cluster-prefixed, additional validations ensure that the topic is part of the active cluster IDs.
    If (even one) topic ID is invalid or unrecognized, the `PRUNE` message is flagged as invalid, and the inspection process is terminated with a failure result. 
    In future we may relax this condition to allow for a certain number of invalid topics, but for now, a single invalid topic results in a failure.
    The inspection process also system keeps track of the topics seen in the `PRUNE` messages of the same RPC. 
    If a topic is repeated (i.e., if there are duplicate topics in the `PRUNE` messages of the same RPC), this is usually a sign of a protocol violation or misbehavior.
    The validation process counts these duplicates and, if the number exceeds a certain threshold, it flags RPC message as invalid and terminates the inspection process with a failure result.
    Note that all `PRUNE` messages on the same (potentially truncated) RPC are validated together, without any sampling, as the number of `PRUNE` messages is usually assumed small, and validating
    them is not assumed to be resource-intensive.
3. `IWANT` messages validation: Each RPC contains one or more `IWANT` messages. Each `IWANT` message contains a list of message IDs that the sender wants from the receiver as the result of an `IHAVE` message.
    The validation process involves iterating through each `IWANT` message received in the (potentially truncated) RPC.
    For each `IWANT` message, the message IDs are validated to ensure they correspond to a valid message ID that recently advertised by the sender in an `IHAVE` message.
    We define an `IWANT` cache miss as the event of an `IWANT` message ID does not correspond to a valid recently advertised `IHAVE` message ID.
    When number of `IWANT` cache misses exceeds a certain threshold, the `IWANT` message is flagged as invalid, and the inspection process is terminated with a failure result.
    The inspection process also system keeps track of the message IDs seen in the `IWANT` messages of the same RPC. 
    If a message ID is repeated (i.e., if there are duplicate message IDs in the `IWANT` messages of the same RPC), this is usually a sign of a protocol violation or misbehavior.
    The validation process counts these duplicates and, if the number exceeds a certain threshold, it flags RPC message as invalid and terminates the inspection process with a failure result.
    Note that all `IWANT` messages on the same (potentially truncated) RPC are validated together, without any sampling, as the number of `IWANT` messages is usually assumed small, and validating 
    them is not assumed to be resource-intensive.
4. `IHAVE` messages validation: Each RPC contains one or more `IHAVE` messages. Each `IHAVE` message contains a list of message IDs that the sender has and is advertising to the receiver.
    The validation process involves iterating through each `IHAVE` message received in the (potentially truncated) RPC.
    Each `IHAVE` message is composed of a topic ID as well as the list of message IDs advertised for that topic.
    Each topic ID is validated to ensure it corresponds to a valid and recognized topic within the Flow-network.
    Topic validation might involve checking if the topic is known, if it's within the scope of the peer's interests or subscriptions, and if it aligns with the network's current configuration (e.g., checking against the active spork ID).
    If the topic is cluster-prefixed, additional validations ensure that the topic is part of the active cluster IDs.
    If (even one) topic ID is invalid or unrecognized, the `IHAVE` message is flagged as invalid, and the inspection process is terminated with a failure result.
    The inspection process also system keeps track of the topics seen in the `IHAVE` messages of the same RPC. When a topic is repeated (i.e., if there are duplicate topics in the `IHAVE` messages of the same RPC), this is usually a sign of a protocol violation or misbehavior.
    The validation process counts these duplicates and, if the number exceeds a certain threshold, it flags RPC message as invalid and terminates the inspection process with a failure result.
    The message IDs advertised in the `IHAVE` messages are also validated ensure there are no duplicates. When a message ID is repeated (i.e., if there are duplicate message IDs in the `IHAVE` messages of the same RPC), this is usually a sign of a protocol violation or misbehavior.
    The validation process counts these duplicates and, if the number exceeds a certain threshold, it flags RPC message as invalid and terminates the inspection process with a failure result.
    Note that all `IHAVE` messages on the same (potentially truncated) RPC are validated together, without any sampling, as the number of `IHAVE` messages is usually assumed small, and validating
    them is not assumed to be resource-intensive.
5. `Publish` messages validation: Each RPC contains a list of `Publish` messages that are intended to be gossiped to the network.
    The validation process involves iterating through each `Publish` message received in the (potentially truncated) RPC.
    To validate the `Publish` messages of an RPC, the inspector samples a subset of the `Publish` messages and validates them for compliance with the Flow protocol semantics.
    This is done to avoid adding excessive computational overhead to the inspection process, as the number of `Publish` messages in an RPC can be large, and validating each message can be resource-intensive.
    The validation of each `Publish` message involves several steps: (1) whether the sender is a valid (staked) Flow node, 
    (2) whether the topic ID is a valid based on the Flow protocol semantics, and (3) whether the local peer has a valid subscription to the topic.
    Failure in any of these steps results in a validation error for the `Publish` message. 
    However, validation error for a single `Publish` message does not cause inspection process to terminate with a failure result for the entire RPC.
    Rather the inspection process continues to validate the rest of the `Publish` messages in the sampled RPC.
    Once the entire sampled RPC is validated, the inspection process is terminated with a success if the number of validation errors is within a certain threshold.
    Otherwise, when the number of validation errors exceeds the threshold, the inspection process is terminated with a failure result, which 
    will cause an _invalid control message notification_ to be sent to the `GossipSubAppSpecificScoreRegistry`, which will then be used for penalizing the sender in the peer scoring system.
    As this is the last step in the inspection process, when an RPC reaches this step, it means that the RPC has passed all the previous inspections and is only being validated for the `Publish` messages.
    Hence, result of this step is used to determine the final result of the inspection process.
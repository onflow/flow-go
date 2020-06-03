# Consensus 

The consensus node is responsible for deciding on the subjective ordering of the transactions that will be executed by the execution nodes. They do so by running a consensus algorithm for the blockchain, whereas each block payload contains an ordered list of collection guarantees received by collection node clusters.

Consensus nodes are also responsible for maintaining the protocol state, which encompasses epochs with their respective set of node identities, as well as the adjudication of all slashing challenges which maintain the economic incentives that serve as the foundation of the bigger Flow system.

This document provides a high-level overview of the consensus node architecture. Each section includes links to the appropriate packages, which may contain more detailed documentation.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->


- [Terminology](#terminology)
- [Processes](#processes)
  - [Collection Guarantee Lifecycle](#collection-guarantee-lifecycle)
  - [Block Seal Lifecycle](#block-seal-lifecycle)
  - [Block Formation](#block-formation)
- [Engines](#engines)
  - [Ingestion](#ingestion)
  - [Propagation](#propagation)
  - [Compliance](#compliance)
  - [Synchronization](#synchronization)
  - [Provider](#provider)
  - [Matching](#matching)
- [Modules](#modules)
  - [Protocol State](#protocol-state)
  - [Block Builder](#block-builder)
  - [Block Finalizer](#block-finalizer)
  - [Core Consensus](#core-consensus)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->


## Terminology

- **Collection** - a set of transactions proposed by a cluster of collection nodes.
- **Guarantee**, also _Collection Guarantee_ - an attestation for a collection guaranteed by a collection cluster, containing a hash over the collection contents and signed by a qualified majority of the participants of the cluster.
- **Result**, also _Execution Result_ - a summary representing the delta of the execution state resulting from the execution of a specific block, containing a reference to the previous state and a state commitment for the resulting state.
- **Receipt**, also _Execution Receipt_ - a proof of execution linked to a specific execution node, containing an execution result, data on execution chunks and a signature of the execution node.
- **Aproval**, also _Result Approval_ - an approval of the execution of a chunk of an execution result, as verified by a specific verification node.
- **Seal**, also _Block Seal_ - an attestation of correct execution of a block in the blockchain, built by the consensus node after receiving the necessary execution receipts and result approvals.
- **Header**, also _Block Header_ - a data structure containing the meta-data for a block, including the merkle root hash for the payload as well as the relevant consensus node signatures.
- **Payload**, also _Block Payload_ - a list of entities included in a block, currently consisting of collection guarantees and block seals.
- **Index**, also _Payload Index_ - a list of entitie IDs included in a block, currently consising of a list of collection guarantee IDs and block seal IDs.
- **Block** - the combination of a block header with a block contents, representing all of the data necessary to construct the and validate the entirety of the block.


## Processes

The consensus nodes accomplish a single high-level process: the formation of and the consensus-finding on blocks with their payloads. Each payload can include collection guarantees and block seals, which are each handled by their own high-level processes.

On an implementation level, each process is implemented by a number of node engines, modules and storage components.

### Collection Guarantee Lifecycle

1. Collection guarantees are received from collection node clusters.
2. Collection guarantees are propagated to other consensus nodes.
3. Collection guarantees are introduced into the memory pool.
4. Collection guarantees are included in a block proposal.
5. Collection guarantees are removed from the memory pool when the block is finalized.

### Block Seal Lifecycle

1. Execution receipts are received from execution nodes.
2. Result approvals are received from verification nodes.
3. Execution receipts are result approvals are introduced into the memory pool.
4. Execution receipts are matched with result approvals for block seal generation.
5. Generated blocks seals are introduced into the memory pool.
6. Execution receipts and result approvals are removed from the memory pool when a seal is generated.
7. Blocks seals are included in a block proposal.
8. Block seals are removed from the memory pool when the block is finalized.

### Block Formation

1. Core consensus algorithm requests a block proposal for a specific parent.
2. Block builder includes collection guarantees available in the memory pool.
3. Block builder includes block seals available in the memory pool.
4. Core consensus algorithm orders finalization of a block.


## Engines

Engines are units of application logic which are generally responsible for a well-isolated process that is part of the bigger system. They receive messages from the network on selected channels and submit messages to the network on the same channels.

### [Ingestion](../../engine/consensus/ingestion)

The `ingestion` engine is responsible for receiving collection guarantees from clusters of collection nodes.

It will perform some basic validation, such as checking the origin of the collection guarantee and the collection expiry. When a collection guarantee appears valid, it forwards it to the propagation engine.

### [Propagation](../../engine/consensus/propagation)

The `propagation` engine is responsible for proactively synchronizing the memory pools for collection guarantee between consensus nodes.

It can receive new collection guarantees from the ingestion engine or directly from other consensus nodes. Once received, it will add it to the memory pool for block construction and share it with other consensus nodes as needed.

### [Compliance](../../engine/consensus/compliance)

The `compliance` engine is responsible for ensuring blocks comply with the validity rules of the protocol state and for enabling communication between the replicas of the core consensus algorithm.

The compliance functions as the communication layer between the replicas of the core consensus algorithm. It receives block proposals and votes from the network and forwards them to the core consensus algorithm, as well as providing the core consensus algorithm with facilities to broadcast block proposals and send votes to the other replicas on the network.

When a block proposal is received, consensus node will first try to assemble all data required for its validation. Once it is available, the compliance engine validates the block proposal against the protocol state, which enforces formation rules outside of the purview of the core consensus algorithm, such as those relating to block payloads. If a block is considered compliant, the header is forwarded to the core consensus algorithm for decision-making.

### [Synchronization](../../engine/common/synchronization)

The `synchronization` engine is responsible for reactive synchronization of consensus nodes about the protocol state.

At regular interval, it will send synchronization requests (pings) to a random subset of consensus nodes, and receive synchonization responses (pongs) in return. If it detects a difference in finalized block height above a certain threshold, it will request the missing block heights.

Additionally, the synchronization engine provides the possibility to request blocks by specific identifier. This is used by the compliance engine to actively request missing blocks that are needed for the validation of another block.

### [Provider](../../engine/consensus/provider)

The `provider` engine is responsible for providing blocks to the non-consensus participants of the Flow system.

At the moment, it simply receives each block proposal from the compliance engine and broadcasts it to all interested nodes on its communication channel.

### [Matching](../../engine/consensus/matching)

The `matching` engine is responsible for receiving execution receipts and result approvals, as well as generating the block seals for them.

Whenever an execution receipt or a result approval are received by the matching engine, it will perform all possible validity checks on it before adding it to the respective memory pool.

If the related result or chunk is new, the matching engine checks if there are any results in its memory pool that can be fully verified with the received result approvals. Once this is the case, it creates the related block seal and adds it to the memory pool for block construction.


## Modules

Modules encapsulate big parts of the application logic, which they provide as a service to other components, such as engines. They usually don't run anything unless another component calls into them.

### [Protocol State](../../state/protocol)

The `protocol state` is the most important stateful component of the consensus nodes. It provides the ability to retrieve a snapshot at a given point of the blockchain history, as well as allowing the mutation of the protocol state by extending it with a block or finalizing a block.

When receiving a block proposal, it is validated against the protocol state by trying to extend the protocol state mutator; this will perform a check for all chain compliance rules, such as some basic header constraints and the full check for payload duplicates or invalid content.

For a lot of messages, we check whether the origin is a valid sender for the given message type. This is done by getting the node identities from the protocol state snapshot and validating the origin against them.

### [Block Builder](../../module/builder/consensus)

The `block builder` is complementary to the protocol state and is responsible for building block proposals that are complying with the validity rules of the protocol state. This means there is a lot of overlap in their logic.

It will retrieve all pending collection guarantees and block seals from the memory pools and check which ones it can validly include into the block payload without breaking compliance on the respective fork of the block chain it is building on.

### [Block Finalizer](../../module/finalizer/consensus)

The `block finalizer` is also complementary to the protocol state and is responsible for providing an idempotent interface for finalizing a block or mapping blocks to their parents.

When provided with a block, it will attempt to finalize each block between the last finalized block and the candidate block in order. If no blocks have to be finalized, it will be an empty operation.

### [Core Consensus](../../consensus/hotstuff)

The `core consensus` in Flow is provided by the HotStuff algorithm. While there are some interdependencies between the components, it exists mostly in isolation from the compliance engine and interacts only through some well-defined interfaces.

Please take a look at the [Hotstuff Documentation](../../consensus/hotstuff/README.md) for a more detailed overview of its internal workings.

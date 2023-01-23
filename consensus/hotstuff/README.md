# Flow's HotStuff

We use a BFT consensus algorithm with deterministic finality in Flow for
* Consensus Nodes: deciding on the content of blocks
* Cluster of Collector Nodes: batching transactions into collections. 

Flow uses a derivative of HotStuff 
(see paper [HotStuff: BFT Consensus in the Lens of Blockchain (version 6)](https://arxiv.org/abs/1803.05069v6) for the original algorithm) called Jolteon(see paper [Jolteon and Ditto: Network-Adaptive Efficient Consensus with Asynchronous Fallback](https://arxiv.org/abs/2106.10362).
Our implementation is a mixture of HotStuff and Jolteon, the main modifications:
* We employ the rules for locking on blocks and block finalization from Jolteon which is built on top of Event-Driven HotStuff. 
Specifically, upon seeing a valid QC, the replica updates its highest known QC. It votes only for blocks that extend that QC(in case of happy path)
or for blocks that contain a valid TC(in case of recovery path).
We finalize a block when it has a 2-chain built on top where with the first chain link being _direct_. This is possible because of active protocol for recovery path.  
* The way we progress though views follows the Jolteon protocol. We advance views when observing a valid QC or TC. A replica only votes or proposes for its current view.  
* Flow's protocol as view synchronization component does not use Passive Pacemaker which was described in the paper, instead we use an Active Pacemaker from Jolteon protocol.
* Flow uses a decentralized random beacon (based on [Dfinity's proposal](https://dfinity.org/pdf-viewer/library/dfinity-consensus.pdf)).
The random beacon is run by Flow's consensus nodes and integrated into the consensus voting process.  

#### Node weights

In Flow, there are multiple HotStuff instances running in parallel. Specifically, the consensus nodes form a HotStuff committee and each collector cluster is its own committee. In the following, we refer to an authorized set of nodes, who run a particular HotStuff instance as a (HotStuff) `committee`.  

* Flow allows nodes to have different weights, reflecting how much they are trusted by the protocol.
  The weight of a node can change over time due to stake changes or discovering protocol violations. 
  A `super-majority` of nodes is defined as a subset of the consensus committee,
  where the nodes have _more_ than 2/3 of the entire committee's accumulated weight.
* The messages from zero-weighted nodes are ignored by all committee members.       
 
 
## Architecture 

#### Determining block validity 

In addition to HotStuff's requirements on block validity, the Flow protocol adds additional requirements. 
For example, it is illegal to repeatedly include the same payload entities (e.g. collections, challenges, etc) in the same fork.
Generally, payload entities expire. However, within the expiry horizon, all ancestors of a block need to be known 
to verify that payload entities are not repeated. 

We exclude the entire logic for determining payload validity from the HotStuff core implementation.
This functionality is encapsulated in the Chain Compliance Layer (CCL) which precedes HotStuff. 
The CCL is designed to forward only fully validated blocks to the HotStuff core logic.
The CCL forwards a block to the HotStuff core logic only if 
* the block's header is valid(including QC),
* the block's payload is valid,
* the block is connected to the most recently finalized block, and
* all ancestors have previously been processed by HotStuff. 

If ancestors of a block are missing, the CCL caches the respective block and (iteratively) requests missing ancestors.

#### Payload generation
Payloads are generated outside the HotStuff core logic. HotStuff only incorporates the payload root hash into the block header. 

#### Structure of votes 
In Flow's HotStuff implementation, votes are used for two purposes: 
1. Prove that a super-majority of committee nodes considers the respective block a valid extension of the chain. 
Therefore, nodes include a `StakingSignature` (BLS with curve BLS12-381) in their vote.  
2. Construct a Source of Randomness as described in [Dfinity's Random Beacon](https://dfinity.org/pdf-viewer/library/dfinity-consensus.pdf).
Therefore, consensus nodes include a `RandomBeaconSignature` (also BLS with curve BLS12-381, used in a threshold signature scheme) in their vote.  

When the primary collects the votes, it verifies content of `SigData` it can contain only a `StakingSignature` or a pair `StakingSignature` + `RandomBeaconSignature`.
A `StakingSignature` must be present in all votes. 
If either signature is invalid, the entire vote is discarded. From all valid votes, the
`StakingSignatures` and the `RandomBeaconSignatures` are aggregated separately.

* Following [version 6 of the HotStuff paper](https://arxiv.org/abs/1803.05069v6), 
replicas forward their votes for block `b` to the leader of the _next_ view, i.e. the primary for view `b.View + 1`. 
* A proposer will attach its own vote for its proposal in the block proposal message 
(instead of signing the block proposal for authenticity and separately sending a vote). 


#### Primary section
For primary section, we use a randomized, weight-proportional selection.


### Component Overview 
HotStuff's core logic is broken down into multiple components. 
The figure below illustrates the dependencies of the core components and information flow between these components.

![](/docs/HotStuffEventLoop.png)

* `MessageHub` is responsible for relaying HotStuff messages, incoming messages are relayed to respective modules depending on message type.
Outgoing messages are relayed to committee using p2p communication. 
* `compliance.Core` is responsible for processing incoming blocks, caching if needed, validating, extending state and publishing to
HotStuff for further processing. 
* `compliance.Engine` performs work scheduling logic for `compliance.Core`, doesn't perform any business logic. 
* `EventLoop` buffers all incoming events, so `EventHandler` can process one event at a time in a single thread.
* `EventHandler` orchestrates all HotStuff components and implements [HotStuff's state machine](/docs/StateMachine.png).
The event handler is designed to be executed single-threaded.
* `SafetyRules` tracks the latest vote, the latest timeout and determines whether to vote for a block and if it's safe to timeout current round. 
* `Pacemaker` is an implementation of Active PaceMaker to ensure liveness by keeping the majority of committee replicas in the same view. Tracks current view.
* `Forks` maintains an in-memory representation of all blocks `b`, whose view is larger or equal to the view of the latest finalized block (known to this specific replica).
As blocks with missing ancestors are cached outside HotStuff (by the Chain Compliance Layer), 
all blocks stored in `Forks` are guaranteed to be connected to the genesis block 
(or the trusted checkpoint from which the replica started). `Forks` is implemented using `LevelledForest`: A blockchain forms a Tree, when removing all blocks
with views strictly smaller than the last finalized block, the chain decomposes into multiple disconnected trees, in graph theory, such structure is a forest, to separate general graph-theoretical 
concepts from the concrete blockchain application, `LevelledForest` refers to blocks as graph `vertices` and to a block's view number as `level`, 
the `LevelledForest` is an in-memory data structure to store and maintain a levelled forest, it provides functions to add vertices, query vertices by their ID (block's hash), query vertices by level, query the children of a vertex, and prune vertices by level (remove them from memory).
`Forks` tracks the finalized blocks and triggers finalization events whenever it observes a valid extension to the chain of finalized blocks.
* `Validator` validates the HotStuff-relevant aspects of
   - QC: total weight of all signers is more than 2/3 of committee weight, validity of signatures, view number is strictly monotonously increasing
   - TC: total weight of all signers is more than 2/3 of committee weight, validity of signatures, proof for entering view. 
   - block proposal: from designated primary for the block's respective view, contains proposer's vote for its own block, QC in block is valid, TC if needed is valid.
   - vote: validity of signature, voter is has positive weight
* `VoteAggregator` caches votes on a per-block basis and builds QC if enough votes have been accumulated.
* `TimeoutAggregator` caches timeouts on a per-view basis and builds TC if enough timeouts have been accumulated. Performs validation and verification of timeouts.
* `Replicas` maintains the list of all authorized network members and their respective weight on a per-view basis. Maintains a static list which changes only between epochs. Furthermore, the `Replicas` contains the primary selection algorithm.
* `DynamicCommittee` maintains the list of all authorized network members and their respective weight on a per-block basis. Extends `Replicas`.
* `BlockProducer` constructs the payload of a block, after the HotStuff core logic has decided which fork to extend 

# Implementation

We have translated the HotStuff protocol into a state machine shown below. The state machine is implemented in `EventHandler`.

![](/docs/StateMachine.png)

#### PaceMaker
The HotStuff state machine interacts with the PaceMaker, which triggers view changes. PaceMaker keeps track of liveness data(newest QC, current view, TC for last view), and updates it when supplied with new data from `EventHandler`. 
Conceptually, the PaceMaker interfaces with the `EventHandler` in two different modes:
* [asynchronous] On timeouts, the PaceMaker will emit a timeout event, which is processed as any other event (such as incoming blocks or votes) through the `EventLoop`.
* [synchronous] When progress is made following the core business logic, the  `EventHandler` will inform the PaceMaker about discovering new QCs or TCs 
via a direct method call (see `PaceMaker interface`). If the PaceMaker changed the view in response, it returns 
a `NewViewEvent` which will be synchronously processed by the  `EventHandler`.

Flow's PaceMaker is an implementation of Active Pacemaker which communicates with other replicas after failing to make progress on happy path. It relies on broadcasting timeout object whenever no progress is made during current round. After collecting timeouts from supermajority of participants replica constructs a TC which can be used to enter next round `V = TC.View + 1`. For calculating round timeouts we use a truncated exponential backoff. We will increase round duration exponentially if no progress is made and exponentially decreasing timeouts on happy path. Protocol tolerates a few failures(`k=6`) before increasing round timeout. This value is the number of rounds we expect during happy path, after this many rounds, we will start increasing timeouts.
The timeout values are limited by lower and upper-bounds to ensure that the PaceMaker can change from large to small timeouts in a reasonable number of views. 
The specific values for lower and upper timeout bounds are protocol-specified; we envision the bounds to be on the order of 1sec (lower bound) and one minute (upper bound).

**Progress**, from the perspective of the PaceMaker is defined as entering view `V`
for which the replica knows a QC or a TC with `V = QC.view + 1` or `V = TC.view + 1`. 
In other words, we transition into the next view due to reaching quorum in the last view.
When comparing to HotStuff, in Jolteon views can be changed only after observing a valid quorum in the last view, otherwise replica is not allowed to change view. 
  
A central, non-trivial functionality of the PaceMaker is to _skip views_. 
Specifically, given a QC or TC with view `V` or , the Pacemaker will skip ahead to view `V + 1` if `currentView ≤ V`.
  
<img src="https://github.com/onflow/flow-go/blob/master/docs/PaceMaker.png" width="200">
 
 

## Code structure

All relevant code implementing the core HotStuff business logic is contained in `/consensus/hotstuff/` (folder containing this README).
When starting to look into the code, we suggest starting with `/consensus/hotstuff/event_loop.go` and `/consensus/hotstuff/event_handler.go`. 


### Folder structure

All files in the `/consensus/hotstuff/` folder, except for `follower_loop.go`, are interfaces for HotStuff-related components. 
The concrete implementations for all HotStuff-relevant components are in corresponding sub-folders. 
For completeness, we list the component implemented in each sub-folder below: 

* `/consensus/hotstuff/blockproducer` builds a block proposal for a specified QC, interfaces with the logic for assembling a block payload, combines all relevant fields into a new block proposal.
* `/consensus/hotstuff/committees` maintains the list of all authorized network members and their respective weight on a per-block and per-view basis depending on implementation; contains the primary selection algorithm. 
* `/consensus/hotstuff/eventloop` buffers all incoming events, so `EventHandler` can process one event at a time in a single thread.
* `/consensus/hotstuff/eventhandler` orchestrates all HotStuff components and implements the HotStuff state machine. The event handler is designed to be executed single-threaded.
* `/consensus/hotstuff/follower` This component is only used by nodes that are _not_ participating in the HotStuff committee. As Flow has dedicated node roles with specialized network functions, only a subset of nodes run the full HotStuff protocol. Nevertheless, all nodes need to be able to act on blocks being finalized. The approach we have taken for Flow is that block proposals are broadcast to all nodes (including non-committee nodes). Non-committee nodes locally determine block finality by applying HotStuff's finality rules. The HotStuff Follower contains the functionality to consume block proposals and trigger downstream processing of finalized blocks. The Follower does not _actively_ participate in HotStuff. 
* `/consensus/hotstuff/forks` maintains an in-memory representation of all blocks `b`, whose view is larger or equal to the view of the latest finalized block (known to this specific HotStuff replica). It tracks the last finalized block and performs finalization of candidate blocks when safety rule is satisfied.   
* `/consensus/hotstuff/helper` contains broadly-used helper functions for testing  
* `/consensus/hotstuff/integration` integration tests for verifying correct interaction of multiple HotStuff replicas
* `/consensus/hotstuff/model` contains the HotStuff data models, including block proposal, vote, timeout, etc. 
Many HotStuff data models are built on top of basic data models defined in `/model/flow/`.
* `/consensus/hotstuff/notifications`: All relevant events within the HotStuff logic are exported though a notification system. Notifications are used by _some_ HotStuff components internally to drive core logic(events from `VoteAggregator` and `TimeoutAggregator`), they also notify other components within the same node of relevant progress and are used for collecting HotStuff metrics.
* `/consensus/hotstuff/pacemaker` contains the implementation of Flow's Active PaceMaker, as described above. Is responsible for protocol liveness. 
* `/consensus/hotstuff/persister` for performance reasons, the implementation maintains the consensus state largely in-memory. The `persister` stores latest safety and liveness data persistently on disk. This allows recovery after a crash without the risk of equivocation.
* `/consensus/hotstuff/safetyrules` tracks the latest vote, the latest timeout and determines whether to vote for a block and if it's safe to timeout current round.
* `/consensus/hotstuff/signature` contains the implementation for threshold signature aggregation for all types of signatures that are used in HotStuff protocol. 
* `/consensus/hotstuff/timeoutaggregator` caches timeouts on a per-view basis and orchestrates processing of timeouts and as result creation of TC.
* `/consensus/hotstuff/timeoutcollector` performs timeout collection and processing, as part of processing validation and verification of timeouts. Builds TC if enough timeouts have been accumulated.
* `/consensus/hotstuff/tracker` implements utility code for tracking the newest QC and TC in multithreaded environment.
* `/consensus/hotstuff/validator` holds the logic for validating the HotStuff-relevant aspects of blocks, QCs, TC, and votes
* `/consensus/hotstuff/verification` contains integration of Flow's cryptographic primitives (signing and signature verification) 
* `/consensus/hotstuff/voteaggregator` caches votes on a per-block basis and builds a QC if enough votes have been accumulated.
* `/consensus/hotstuff/votecollector` performs vote collection and processing, and builds a QC if enough votes have been accumulated.

## Pending Extensions  
* Switch to different format of signatures for votes.

## Telemetry

The HotStuff state machine exposes some details about its internal progress as notification through the `hotstuff.Consumer`.
The following figure depicts at which points notifications are emitted. 

![](/docs/StateMachine_wirth_notifications.png)

We have implemented a telemetry system (`hotstuff.notifications.TelemetryConsumer`) which implements the `Consumer` interface.
The `TelemetryConsumer` tracks all events as belonging together that were emitted during a path through the state machine as well as events from components that perform asynchronous processing(`VoteAggregator`, `TimeoutAggregator`).
Each `path` through the state machine is identified by a unique id.
Generally, the `TelemetryConsumer` could export the collected data to a variety of backends.
For now, we export the data to a logger.

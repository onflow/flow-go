# Flow's HotStuff

We use a BFT consensus algorithm with deterministic finality in Flow for
* Consensus Nodes: decide on the content of blocks by including collections of transactions,
* Cluster of Collector Nodes: batch transactions into collections. 

Conceptually, Flow uses a derivative of [HotStuff](https://arxiv.org/abs/1803.05069) called Jolteon.
It was originally described in the paper ['Jolteon and Ditto: Network-Adaptive Efficient Consensus with Asynchronous Fallback'](https://arxiv.org/abs/2106.10362),
published June 2021 by Meta’s blockchain research team Novi and academic collaborators.
Meta’s team (then called 'Diem') [implemented](https://github.com/diem/diem/blob/latest/consensus/README.md) Jolteon with marginal modifications and named it
[DiemBFT v4](https://developers.diem.com/papers/diem-consensus-state-machine-replication-in-the-diem-blockchain/2021-08-17.pdf),
which was subsequently rebranded as [AptosBFT](https://github.com/aptos-labs/aptos-core/tree/main/developer-docs-site/static/papers/aptos-consensus-state-machine-replication-in-the-aptos-blockchain).
Conceptually, Jolteon (DiemBFT v4, AptosBFT) belongs to the family of HotStuff consensus protocols,
but adds two significant improvements over the original HotStuff: (i) Jolteon incorporates a PaceMaker with active message exchange for view synchronization
and (ii) utilizes the most efficient 2-chain commit rule. 

The foundational innovation in the original HotStuff was its pipelining of block production and finalization.
It utilizes leaders for information collection and to drive consensus, which makes it highly message-efficient. 
In HotStuff, the consensus mechanics are cleverly arranged such that the protocol runs as fast as network conditions permit.
(Not be limited by fixed, minimal wait times for messages.)
This property is called "responsiveness" and is very important in practise.  
HotStuff is a round-based consensus algorithm, which requires a supermajority of nodes to be in the same view to make progress. It is the role
of the pacemaker to guarantee that eventually a supermajority of nodes will be in the same view. In the original HotStuff, the pacemaker was
essentially left as a black box. The only requirement was that the pacemaker had to get the nodes eventually into the same view. 
Vanilla HotStuff requires 3 subsequent children to finalize a block on the happy path (aka '3-chain rule').
In the original HotStuff paper, the authors discuss the more efficient 2-chain rule. They explain a timing-related edge-case, where the protocol
could theoretically get stuck in a timeout loop without progress. To guarantee liveness despite this edge case, the event-driven HotStuff variant in the original paper employs
the 3-chain rule. 

As this discussion illustrates, HotStuff is more a family of algorithms: the pacemaker is conceptually separated and can be implemented
in may ways. The finality rule is easily swapped out. In addition, there are various other degrees of freedom left open in the family
of HotStuff protocols. 
The Jolteon protocol extends HotStuff by specifying one particular pacemaker, which utilizes dedicated messages to synchronize views and provides very strong guarantees.
Thereby, the Jolteon pacemaker closes the timing edge case forcing the original HotStuff to use the 3-chain rule for liveness.
As a consequence, the Jolteon protocol can utilize the most efficient 2-chain rule. 
While Jolteon's close integration of the pacemaker into the consensus meachanics changes the correctness and liveness proofs significantly,
the protocol's runtime behaviour matches the HotStuff framework. Therefore, we categorize Jolteon, DiemBFT v4, and AptosBFT as members of the HotStuff family.

Flow's consensus is largely an implementation of Jolteon with some elements from DiemBFT v4. While these consensus protocols are identical on a conceptual level,
they subtly differ in the information included into the different consensus messages. For Flow's implementation, we combined nuances
from Jolteon and DiemBFT v4 as follows to improve runtime efficiency, reduce code complexity, and minimize the surface for byzantine attacks: 

 
* Flow's `TimeoutObject` implements the `timeout` message in Jolteon. 
  * In the Jolteon protocol, 
  the `timeout` message contains the view `V`, which the sender wishes to abandon and the latest
  Quorum Certificate [QC] known to the sender. Due to successive leader failures, the QC might
  not be from the previous view, i.e. `QC.View + 1 < V` is possible. When receiving a `timeout` message, 
  it is possible for the recipient to advance to round `QC.View + 1`, 
  but not necessarily to view `V`, as a malicious sender might set `V` to an erroneously large value.
  On the one hand, a recipient that has fallen behind cannot catch up to view `V` immediately. On the other hand, the recipient must
  cache the timeout to guarantee liveness, making it vulnerable to memory exhaustion attacks.
  * [DiemBFT v4](https://developers.diem.com/papers/diem-consensus-state-machine-replication-in-the-diem-blockchain/2021-08-17.pdf) introduced the additional rule that the `timeout`
  must additionally include the Timeout Certificate [TC] for the previous view,
  if and only if the contained QC is _not_ from the previous round (i.e. `QC.View + 1 < V`). Conceptually,
  this means that the sender of the timeout message must prove that they entered round `V` according to protocol rules.
  In other words, malicious nodes cannot send timeouts for future views that they should not have entered. 

  For Flow, we follow the convention from DiemBFT v4. This modification simplifies byzantine-resilient processing of `TimeoutObject`s,
  avoiding subtle spamming and memory exhaustion attacks. Furthermore, it speeds up the recovery of crashed nodes.

* For consensus votes, we stick with the original Jolteon format, i.e. we do _not_ include the highest QC known to the voter, which is the case in DiemBFT v4.
  The QC is useful only on the unhappy path, where a node has missed some recent blocks. However, including a QC in every vote adds consistent overhead to the happy
  path. In Jolteon as well as DiemBFT v4, the timeout messages already contain the highest known QCs. Therefore, the highest QC is already
  shared among the network in the unhappy path even without including it in the votes. 
* In Jolteon, the TC contains the full QCs from a supermajority of nodes, which have some overhead in size. DiemBFT v4 improves this by only including
  the QC's respective views in the TC. Flow utilizes this optimization from DiemBFT v4.  

In the following, we will use the terms Jolteon and HotStuff interchangeably to refer to Flow's consensus implementation.
Beyond the Realm of HotStuff and Jolteon, we have added the following advancement to Flow's consensus system:

* Flow contains a **decentralized random beacon** (based on [Dfinity's proposal](https://dfinity.org/pdf-viewer/library/dfinity-consensus.pdf)).
  The random beacon is run by Flow's consensus nodes and integrated into the consensus voting process. The random beacon provides a nearly
  unbiasable source of entropy natively within the protocol that is verifiable and deterministic. The random beacon can be used to generate pseudo random
  numbers, which we use within Flow protocol in various places. We plan to also use the random beacon to implement secure pseudo random number generators in Candence.     

 
## Architecture 

_Concepts and Terminology_

In Flow, there are multiple HotStuff instances running in parallel. Specifically, the consensus nodes form a HotStuff committee and each collector cluster is its own committee.
In the following, we refer to an authorized set of nodes, who run a particular HotStuff instance as a (HotStuff) `committee`.

* Flow allows nodes to have different weights, reflecting how much they are trusted by the protocol.
  The weight of a node can change over time due to stake changes or discovering protocol violations.
  A `super-majority` of nodes is defined as a subset of the consensus committee,
  where the nodes have _more_ than 2/3 of the entire committee's accumulated weight.
* Conceptually, Flow allows that the random beacon is run only by a subset of consensus nodes, aka the "random beacon committee". 
* The messages from zero-weighted nodes are ignored by all committee members.


### Determining block validity 

In addition to Jolteon's requirements on block validity, the Flow protocol adds additional requirements. 
For example, it is illegal to repeatedly include the same payload entities (e.g. collections, challenges, etc) in the same fork.
Generally, payload entities expire. However, within the expiry horizon, all ancestors of a block need to be known 
to verify that payload entities are not repeated. 

We exclude the entire logic for determining payload validity from the HotStuff core implementation.
This functionality is encapsulated in the Chain Compliance Layer (CCL) which precedes HotStuff. 
The CCL is designed to forward only fully validated blocks to the HotStuff core logic.
The CCL forwards a block to the HotStuff core logic only if 
* the block's header is valid (including QC and optional TC),
* the block's payload is valid,
* the block is connected to the most recently finalized block, and
* all ancestors have previously been forwarded to HotStuff. 

If ancestors of a block are missing, the CCL caches the respective block and (iteratively) requests missing ancestors.

### Payload generation
Payloads are generated outside the HotStuff core logic. HotStuff only incorporates the payload root hash into the block header. 

### Structure of votes 
In Flow's HotStuff implementation, votes are used for two purposes: 
1. Prove that a super-majority of committee nodes consider the respective block a valid extension of the chain. 
Therefore, nodes include a `StakingSignature` (BLS with curve BLS12-381) in their vote.  
2. Construct a Source of Randomness as described in [Dfinity's Random Beacon](https://dfinity.org/pdf-viewer/library/dfinity-consensus.pdf).
Therefore, consensus nodes include a `RandomBeaconSignature` (also BLS with curve BLS12-381, used in a threshold signature scheme) in their vote.  

When the primary collects the votes, it verifies the content of `SigData`, which can contain only a `StakingSignature` or a pair `StakingSignature` + `RandomBeaconSignature`.
A `StakingSignature` must be present in all votes. (There is an optimization already implemented in the code, making the `StakingSignature` optional, but it is not enabled.) 
If either signature is invalid, the entire vote is discarded. From all valid votes, the
`StakingSignatures` and the `RandomBeaconSignatures` are aggregated separately.

For purely consensus-theoretical purposes, it would be sufficient to use a threshold signature scheme. However, thresholds signatures have the following two 
important limitations, for which reason Flow uses aggregated signatures in addition: 
* The threshold signature carries no information about who signed. Meaning with the threshold signature alone, we have to way to distinguish
  the nodes are contributing from the ones being offline. The mature flow protocol will reward nodes based on their contributions to QCs,
  which requires a conventional aggregated signature.
* Furthermore, the distributed key generation [DKG] for threshold keys currently limits the number of nodes. By including a signature aggregate,
  we can scale the consensus committee somewhat beyond the limitations of the [DKG]. Then, the nodes contributing to the random beacon would only
  be a subset of the entire consensus committee. 

### Communication topology

* Following [version 6 of the HotStuff paper](https://arxiv.org/abs/1803.05069v6), 
replicas forward their votes for block `b` to the leader of the _next_ view, i.e. the primary for view `b.View + 1`. 
* A proposer will attach its own vote for its proposal in the block proposal message 
(instead of signing the block proposal for authenticity and separately sending a vote). 


### Primary section
For primary section, we use a randomized, weight-proportional selection.


## Implementation Components 
HotStuff's core logic is broken down into multiple components. 
The figure below illustrates the dependencies of the core components and information flow between these components.

![](/docs/ComponentInteraction.png)
<!--- source: https://drive.google.com/file/d/1rZsYta0F9Uz5_HM84MlMmMbiR62YALX-/ -->

* `MessageHub` is responsible for relaying HotStuff messages. Incoming messages are relayed to the respective modules depending on their message type.
Outgoing messages are relayed to the committee though the networking layer via epidemic gossip ('broadcast') or one-to-one communication ('unicast').
* `compliance.Engine` is responsible for processing incoming blocks, caching if needed, validating, extending state and forwarding them to HotStuff for further processing.
  Note: The embedded `compliance.Core` component is responsible for business logic and maintaining state; `compliance.Engine` schedules work and manages worker threads for the `Core`.
* `EventLoop` buffers all incoming events. It manages a single worker routine executing the EventHandler`'s logic.
* `EventHandler` orchestrates all HotStuff components and implements the [HotStuff's state machine](/docs/StateMachine.png).
The event handler is designed to be executed single-threaded.
* `SafetyRules` tracks the latest vote, the latest timeout and determines whether to vote for a block and if it's safe to timeout current round. 
* `Pacemaker` implements Jolteon's PaceMaker.  It manages and updates a replica's local view and synchronizes it with other replicas.
  The `Pacemaker` ensures liveness by keeping a supermajority of the committee in the same view. 
* `Forks` maintains an in-memory representation of all blocks `b`, whose view is larger or equal to the view of the latest finalized block (known to this specific replica).
  As blocks with missing ancestors are cached outside HotStuff (by the Chain Compliance Layer), 
  all blocks stored in `Forks` are guaranteed to be connected to the genesis block 
  (or the trusted checkpoint from which the replica started). `Forks` tracks the finalized blocks and triggers finalization events whenever it
  observes a valid extension to the chain of finalized blocks. `Forks` is implemented using `LevelledForest`:
  - Conceptually, a blockchain constructs a tree of blocks. When removing all blocks
    with views strictly smaller than the last finalized block, this graph decomposes into multiple disconnected trees (referred to as a forest in graph theory).
    `LevelledForest` is an in-memory data structure to store and maintain a levelled forest. It provides functions to add vertices, query vertices by their ID
    (block's hash), query vertices by level (block's view), query the children of a vertex, and prune vertices by level (remove them from memory).
    To separate general graph-theoretical concepts from the concrete blockchain application, `LevelledForest` refers to blocks as graph `vertices`
    and to a block's view number as `level`.
* `Validator` validates the HotStuff-relevant aspects of
   - QC: total weight of all signers is more than 2/3 of committee weight, validity of signatures, view number is strictly monotonously increasing;
   - TC: total weight of all signers is more than 2/3 of committee weight, validity of signatures, proof for entering view;
   - block proposal: from designated primary for the block's respective view, contains proposer's vote for its own block, QC in block is valid,
     a valid TC for the previous view is included if and only if the QC is not for the previous view;
   - vote: validity of signature, voter is has positive weight.
* `VoteAggregator` caches votes on a per-block basis and builds QC if enough votes have been accumulated.
* `TimeoutAggregator` caches timeouts on a per-view basis and builds TC if enough timeouts have been accumulated. Performs validation and verification of timeouts.
* `Replicas` maintains the list of all authorized network members and their respective weight, queryable by view. 
  It maintains a static list, which changes only between epochs. Furthermore, `Replicas` knows the primary for each view.
* `DynamicCommittee` maintains the list of all authorized network members and their respective weight on a per-block basis.
  It extends `Replicas` allowing for committee changes mid epoch, e.g. due to slashing or node ejection. 
* `BlockProducer` constructs the payload of a block, after the HotStuff core logic has decided which fork to extend 

# Implementation

We have translated the HotStuff protocol into the state machine shown below. The state machine is implemented in `EventHandler`.

![](/docs/StateMachine.png)
<!--- source: https://drive.google.com/file/d/1la4jxyaEJJfip7NCWBM9YBTz6PK4-N9e/ -->


#### PaceMaker
The HotStuff state machine interacts with the PaceMaker, which triggers view changes. The PaceMaker keeps track of
liveness data (newest QC, current view, TC for last view), and updates it when supplied with new data from `EventHandler`. 
Conceptually, the PaceMaker interfaces with the `EventHandler` in two different modes:
* [asynchronous] On timeouts, the PaceMaker will emit a timeout event, which is processed as any other event (such as incoming blocks or votes) through the `EventLoop`.
* [synchronous] When progress is made following the core business logic, the  `EventHandler` will inform the PaceMaker about discovering new QCs or TCs 
via a direct method call (see `PaceMaker interface`). If the PaceMaker changed the view in response, it returns 
a `NewViewEvent` which will be synchronously processed by the  `EventHandler`.

Flow's PaceMaker utilizes dedicated messages for synchronizing the consensus participant's local views.
It broadcasts a `TimeoutObject`,  whenever no progress is made during the current round. After collecting timeouts from a supermajority of participants,
the replica constructs a TC which can be used to enter the next round `V = TC.View + 1`. For calculating round timeouts we use a truncated exponential backoff.
We will increase round duration exponentially if no progress is made and exponentially decrease timeouts on happy path.
During normal operation with some benign crash failures,  a small number of `k` subsequent leader failures is expected.
Therefore, our PaceMaker tolerates a few failures (`k=6`) before starting to increase timeouts, which is valuable for quickly
skipping over the offline replicase. However, the probability of `k` subsequent leader failures decreases exponentially with `k` (due to Flow's randomized leader selection).
Therefore, beyond `k=6`, we start increasing timeouts.
The timeout values are limited by lower and upper-bounds to ensure that the PaceMaker can change from large to small timeouts in a reasonable number of views. 
The specific values for lower and upper timeout bounds are protocol-specified; we envision the bounds to be on the order of 1sec (lower bound) and one minute (upper bound).

**Progress**, from the perspective of the PaceMaker, is defined as entering view `V`
for which the replica knows a QC or a TC with `V = QC.view + 1` or `V = TC.view + 1`. 
In other words, we transition into the next view when observing a quorum from the last view.
In contrast to HotStuff, Jolteon only allows a transition into view `V+1` after observing a valid quorum for view `V`. There is no other, passive method for honest nodes to change views.
  
A central, non-trivial functionality of the PaceMaker is to _skip views_. 
Specifically, given a QC or TC with view `V`, the Pacemaker will skip ahead to view `V + 1` if `currentView ≤ V`.

![](/docs/PaceMaker.png)
 <!--- source: https://drive.google.com/file/d/1la4jxyaEJJfip7NCWBM9YBTz6PK4-N9e/ -->
 

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
* `/consensus/hotstuff/forks` maintains an in-memory representation of blocks, whose view is larger or equal to the view of the latest finalized block (known to this specific HotStuff replica). Per convention, all blocks stored in `forks` passed validation and their ancestry is fully known. `forks` tracks the last finalized block and implements the 2-chain finalization rule. Specifically, we finalize block `B`, if a _certified_ child `B'` is known that was produced in the view `B.View +1`.
* `/consensus/hotstuff/helper` contains broadly-used helper functions for testing  
* `/consensus/hotstuff/integration` integration tests for verifying correct interaction of multiple HotStuff replicas
* `/consensus/hotstuff/model` contains the HotStuff data models, including block proposal, vote, timeout, etc. 
  Many HotStuff data models are built on top of basic data models defined in `/model/flow/`.
* `/consensus/hotstuff/notifications`: All relevant events within the HotStuff logic are exported though a notification system. Notifications are used by _some_ HotStuff components internally to drive core logic (e.g. events from `VoteAggregator` and `TimeoutAggregator` can trigger progress in the `EventHandler`). Furthermore, notifications inform other components within the same node of relevant progress and are used for collecting HotStuff metrics. Per convention, notifications are idempotent. 
* `/consensus/hotstuff/pacemaker` contains the implementation of Flow's Active PaceMaker, as described above. Is responsible for protocol liveness. 
* `/consensus/hotstuff/persister` stores the latest safety and liveness data _synchronously_ on disk.
  The `persister` only covers the minimal amount of data that is absolutely necessary to avoid equivocation after a crash.
  This data must be stored on disk immediately whenever updated, before the node can progress with its consensus logic.
  In comparison, the majority of the consensus state is held in-memory for performance reasons and updated in an eventually consistent manner.
  After a crash, some of this data might be lost (but can be re-requested) without risk of protocol violations.
* `/consensus/hotstuff/safetyrules` tracks the latest vote and the latest timeout.
  It determines whether to vote for a block and if it's safe to construct a timeout for the current round.
* `/consensus/hotstuff/signature` contains the implementation for threshold signature aggregation for all types of signatures that are used in HotStuff protocol. 
* `/consensus/hotstuff/timeoutcollector` encapsulates the logic for validating timeouts for one particular view and aggregating them to a TC. 
* `/consensus/hotstuff/timeoutaggregator` orchestrates  the `TimeoutCollector`s for different views. It distributes timeouts to the respective  `TimeoutCollector` and prunes collectors that are no longer needed. 
* `/consensus/hotstuff/tracker` implements utility code for tracking the newest QC and TC in a multithreaded environment.
* `/consensus/hotstuff/validator` holds the logic for validating the HotStuff-relevant aspects of blocks, QCs, TC, and votes
* `/consensus/hotstuff/verification` contains integration of Flow's cryptographic primitives (signing and signature verification) 
* `/consensus/hotstuff/votecollector` encapsulates the logic for caching, validating, and aggregating votes for one particular view.
  It tracks, whether a valid proposal for view is known and when enough votes have been collected, it builds a QC.
* `/consensus/hotstuff/voteaggregator` orchestrates  the `VoteCollector`s for different views. It distributes votes to the respective  `VoteCollector`, notifies the `VoteCollector` about the arrival of their respective block, and prunes collectors that are no longer needed.


## Telemetry

The HotStuff state machine exposes some details about its internal progress as notification through the `hotstuff.Consumer`.
The following figure depicts at which points notifications are emitted. 

![](/docs/StateMachine_with_notifications.png)
 <!--- source: https://drive.google.com/file/d/1la4jxyaEJJfip7NCWBM9YBTz6PK4-N9e/ -->

We have implemented a telemetry system (`hotstuff.notifications.TelemetryConsumer`) which implements the `Consumer` interface.
The `TelemetryConsumer` tracks all events as belonging together that were emitted during a path through the state machine as well as events from components that perform asynchronous processing (`VoteAggregator`, `TimeoutAggregator`).
Each `path` through the state machine is identified by a unique id.
Generally, the `TelemetryConsumer` could export the collected data to a variety of backends.
For now, we export the data to a logger.

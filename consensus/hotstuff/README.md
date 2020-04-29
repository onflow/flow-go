# Flow's HotStuff

We use a BFT consensus algorithm with deterministic finality in Flow for
* Consensus Nodes: deciding on the content of blocks
* Cluster of Collector Nodes: batching transactions into collections. 

Flow uses a derivative of HotStuff 
(see paper [HotStuff: BFT Consensus in the Lens of Blockchain (version 6)](https://arxiv.org/abs/1803.05069v6) for the original algorithm).
Our implementation is a mixture of Chained HotStuff and Event-Driven HotStuff with a few additional, minor modifications:
* We employ the rules for locking on blocks and block finalization from Event-Driven HotStuff. 
Specifically, we lock the newest block which has an (direct or indirect) 2-chain built on top. 
We finalize a block when it has a 3-chain built on top where with the first two chain links being _direct_.  
* The way we progress though views follows the Chained HotStuff algorithm. 
A replica only votes or proposes for its current view.  
* Flow's HotStuff does not implement `ViewChange` messages
(akin to the approach taken in [Streamlet:  Textbook Streamlined Blockchains](https://eprint.iacr.org/2020/088.pdf))
* Flow also uses a decentralized random beacon (based on [Dfinity's proposal](https://dfinity.org/pdf-viewer/library/dfinity-consensus.pdf)).
The random beacon is run by Flow's consensus nodes and integrated into the consensus voting process.  
* Flow allows for consensus nodes to be differently staked and their stake to change over time (e.g. due to slashing).
A super-majority of consensus nodes is defined as a subset of consensus nodes which hold _more_ than 2/3 of the entire consensus stake.   
 
## Architecture 

#### Determining block validity 

In addition to HotStuff's requirements on block validity, the Flow protocol adds additional requirements. 
For example, it is illegal to repeatedly include the same payload entities, such as collections, challenges, in the same fork.
Generally, payload entities expire. However within the expiry horizon, all ancestors of a block need to be known 
to verify that payload entities are not repeated. 

We exclude the entire logic for determining payload validity from the HotStuff core implementation. 
This functionality is encapsulated in the Chain Compliance Layer (CCL) which precedes HotStuff. 
The CCL forwards a block to the HotStuff core logic only if 
* the block's payload is valid,
* the block is connected to the most recently finalized block, and
* all ancestors have previously been processed by HotStuff. 

If ancestors of a block are missing, the CCL caches the respective block and (iteratively) requests missing ancestors.

#### Payload generation
Payloads are generated outside of the HotStuff core logic. HotStuff only incorporates the payload root hash into the block header. 

#### Structure of consensus votes 
In Flow's HotStuff implementation, consensus nodes' votes are used for two purposes: 
1. Prove that consensus nodes holding a super-majority of consensus stake approved of the respective block. 
Therefore, consensus nodes include a `StakingSignature` (BLS with curve BLS12381) in their vote.  
2. Construct a Source of Randomness as described in [Dfinity's Random Beacon](https://dfinity.org/pdf-viewer/library/dfinity-consensus.pdf).
Therefore, consensus nodes include a `RandomBeaconSignature` (BLS based on Joint-Feldman DKG for threshold signatures) in their vote.  

When the primary collects the votes, it verifies the `StakingSignature` and the `RandomBeaconSignature`.
If either signature is invalid, the entire vote is discarded. From all valid votes, the
`StakingSignatures` and the `RandomBeaconSignatures` are aggregated separately. 
(note: Only the aggregation of the `RandomBeaconSignature` into a threshold signature is currently implemented. 
Aggregation of the BLS `StakingSignatures` will be added later.)

* Following [version 6 of the HotStuff paper](https://arxiv.org/abs/1803.05069v6), 
replicas forward their votes for block `b` to the leader of the _next_ view, i.e. the primary for view `b.view + 1`. 
* A proposer will attach its own vote for its proposal in the block proposal message 
(instead of signing the block proposal for authenticity and sending a separate vote). 


#### Primary section
For primary section, Flow currently uses a simply round robin scheme, without considering stake.
We plan to upgrade this to randomized, stake-proportional selection later.


### Component Overview 
HotStuff's core logic is broken down into multiple components. 
The figure below illustrates the dependencies of the components and information flow between the components.

![](/docs/HotStuffEventLoop.png)

* `HotStuffEngine` and `ChainComplianceLayer` do not contain any core consensus logic. 
They are responsible for relaying consensus messages and validating consensus payloads respectively. 
* `EventLoop` buffers all incoming events, so `EventHandler` can process one event at a time in a single thread.
* `EventHandler` orchestrates all HotStuff components and implements the [HotStuff's state machine](/docs/StateMachine.png).
The event handler is designed to be executed single-threaded. 
* `NetworkSender` relays outgoing consensus messages 
* `Pacemaker` is a basic PaceMaker for keeping the majority of consensus replicas in the same view
* `Forks` maintains an in-memory representation of all blocks `b`, whose view is larger or equal to the view of the latest finalized block (known to this specific consensus replica).
As blocks with missing ancestors are cached outside of HotStuff (by the Chain Compliance Layer), 
all blocks stored in `Forks` are guaranteed to be connected to the genesis block 
(or the trusted checkpoint from which the replica started). Internally, `Forks` consists of multiple layers:
   - `LevelledForest`: A blockchain forms a Tree. When removing all blocks with views strictly smaller than the last finalized block, the chain decomposes into multiple disconnected trees. In graph theory, such structure is a forrest. To separate general graph-theoretical concepts from the concrete block-chain application, `LevelledForest` refers to blocks as graph `vertices` and to a block's view number as `level`. The `LevelledForest` is an in-memory data structure to store and maintain a leveled forrest. It provides functions to add vertices, query vertices by their ID (block's hash), query vertices by level, query the children of a vertex, and prune vertices by level (remove them from memory).
   - `Finalizer`: the finalizer uses the `LevelledForest` to store blocks. The Finalizer tracks the locked and finalized blocks. Furthermore, it evaluates whether a block is safe to vote for.
   - `ForkChoice`: implements the fork choice rule. Currently, `NewestForkChoice` implements the freshest fork choice rule.
   - `Forks` is the highest level. It bundles the `Finalizer` and `ForkChoice` into one component.
* `validator` validates the HotStuff-relevant aspects of
   - QC: total stake of all signers is more than 2/3 of consensus stake, validity of signatures, view number is strictly monotonously increasing
   - block proposal: from expected proposer, contains proposer's vote for its own block, QC in block is valid
   - vote: validity of signature and voter is staked 
* `voteaggregator` caches votes on a per-block basis and builds qc if enough votes have been accumulated.
* `voter` tracks the view of the latest vote and determines whether or not to vote for a block (by calling `forks.IsSafeBlock`)
* `members` maintains the list of all authorized network members and their respective stake on a per-block basis.

# Implementation

We have translated the Chained HotStuff protocol into a state machine shown below. The implementation of this state machine 
is in `EventHandler`.

![](/docs/StateMachine.png)

The HotStuff state machine interacts with the PaceMaker, which triggers view changes. Conceptually, the PaceMaker
interfaces with the `EventHandler` in two different modes:
* [asynchronous] On timeouts, the PaceMaker will emit a timeout event, which is processed as any other event (such as incoming blocks or votes) through the `EventLoop`.
* [synchronous] When progress is made following the happy-path business logic, the  `EventHandler` will inform the PaceMaker about completing the respective 
processing step via a direct method call (see `PaceMaker interface`). If the PaceMaker changed the view in response, it returns 
a `NewViewEvent` which will be synchronously processed by the  `EventHandler`.

Flow's PaceMaker is simple (compared to Libra's PaceMaker for instance). It solely relies on increasing the timeouts 
exponentially if no progress is made and linearly decreasing timeouts on the happy path. 
The timeout values are limited by lower and upper-bounds to ensure that the PaceMaker can change from large to small timeouts in a reasonable number of views. 
The specific values for lower and upper timeout bounds are protocol-specified; we envision the bounds to be on the order of 1sec (lower bound) and multiple days (upper bound).
  
A central, non-trivial functionality of the PaceMaker is to _skip views_. 
Specifically, given a QC with view `qc.view`, the Pacemaker will skip ahead to view `qc.view + 1` if `currentView ≤ qc.view`.
  
<img src="https://github.com/dapperlabs/flow-go/blob/79f48bf9fe067778a19f89f6b9b5ee03972b6c78/docs/PaceMaker.png" width="200">
 
 

## Code structure

All relevant code implementing the core HotStuff business logic is contained in `/consensus/hotstuff/` (folder containing this README).
When starting to look into the code, we suggest starting with `/consensus/hotstuff/event_loop.go` and `/consensus/hotstuff/event_handler.go`. 


### Folder structure

All files in the `/consensus/hotstuff/` folder, except for `event_loop.go`, are interfaces for HotStuff-related components. 
The concrete implementations for all HotStuff-relevant components are in corresponding sub-folders. 
For completeness, we list the component implemented in each subfolder below: 

* `/consensus/hotstuff/blockproducer` builds a block proposal for a specified QC, interfaces with the logic for assembling a block payload, combines all relevant fields into a new block proposal.
* `/consensus/hotstuff/eventhandler` orchestrates all HotStuff components and implements the HotStuff state machine. The event handler is designed to be executed single-threaded.
* `/consensus/hotstuff/follower` This component is only used by non-consensus nodes. As Flow has dedicated node roles with specialized network functions, only the Consensus Nodes run the full HotStuff protocol. Nevertheless, all nodes need to be able to act on blocks being finalized. The approach we have taken for Flow is that block proposals are broadcast to all nodes (including non-consensus nodes). Non-consensus nodes locally determine block finality by applying HotStuff's finality rules. The HotStuff Follower contains the functionality to consume block proposals and trigger downstream processing of finalized blocks. The Follower does not _actively_ participate in consensus. 
* `/consensus/hotstuff/forks` maintains an in-memory representation of all blocks `b`, whose view is larger or equal to the view of the latest finalized block (known to this specific consensus replica). 
It tracks the last finalized block, the currently locked block, evaluates whether it is safe to vote for a given block and provides a Fork-Choice when the replica is primary.  
* `/consensus/hotstuff/model` contains the HotStuff data models, including block proposal, QC, etc. 
Many HotStuff data models are built on top of basic data models defined in `/model/flow/`.
* `/consensus/hotstuff/notifications`: All relevant events within the HotStuff logic are exported though a notification system. While the notifications are _not_ used HotStuff-internally, they notify other components within the same node of relevant progress and are used for collecting consensus metrics.
* `/consensus/hotstuff/pacemaker` contains the implementation of Flow's basic PaceMaker, as described above.
* `/consensus/hotstuff/signature` contains integration of Flow's cryptography. 
* `/consensus/hotstuff/validator` holds the logic for validating the HotStuff-relevant aspects of blocks, QCs, and votes
* `/consensus/hotstuff/members` maintains the list of all authorized network members and their respective stake on a per-block basis.
* `/consensus/hotstuff/voteaggregator` caches votes on a per-block basis and builds a QC if enough votes have been accumulated.
* `/consensus/hotstuff/voter` tracks the view of the latest vote and determines whether or not to vote for a block


## Pending Extensions  
* BLS Aggregation of the `StakingSignatures`
* upgrade primary section to randomized, stake-proportional selection
* include Epochs 
* upgrade PaceMaker to include Timeout Certificates 
* refactor crypto integration (code in `signature`) for better auditability


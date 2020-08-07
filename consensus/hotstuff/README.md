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

#### Structure of votes 
In Flow's HotStuff implementation, votes are used for two purposes: 
1. Prove that a super-majority of committee nodes considers the respective block a valid extension of the chain. 
Therefore, nodes include a `StakingSignature` (BLS with curve BLS12381) in their vote.  
2. Construct a Source of Randomness as described in [Dfinity's Random Beacon](https://dfinity.org/pdf-viewer/library/dfinity-consensus.pdf).
Therefore, consensus nodes include a `RandomBeaconSignature` (BLS based on Joint-Feldman DKG for threshold signatures) in their vote.  

When the primary collects the votes, it verifies the `StakingSignature` and the `RandomBeaconSignature`.
If either signature is invalid, the entire vote is discarded. From all valid votes, the
`StakingSignatures` and the `RandomBeaconSignatures` are aggregated separately. 
(note: Only the aggregation of the `RandomBeaconSignature` into a threshold signature is currently implemented. 
Aggregation of the BLS `StakingSignatures` will be added later.)

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

* `HotStuffEngine` and `ChainComplianceLayer` do not contain any core HotStuff logic. 
They are responsible for relaying HotStuff messages and validating block payloads respectively. 
* `EventLoop` buffers all incoming events, so `EventHandler` can process one event at a time in a single thread.
* `EventHandler` orchestrates all HotStuff components and implements [HotStuff's state machine](/docs/StateMachine.png).
The event handler is designed to be executed single-threaded. 
* `Communicator` relays outgoing HotStuff messages (votes and block proposals)
* `Pacemaker` is a basic PaceMaker to ensure liveness by keeping the majority of committee replicas in the same view
* `Forks` maintains an in-memory representation of all blocks `b`, whose view is larger or equal to the view of the latest finalized block (known to this specific replica).
As blocks with missing ancestors are cached outside of HotStuff (by the Chain Compliance Layer), 
all blocks stored in `Forks` are guaranteed to be connected to the genesis block 
(or the trusted checkpoint from which the replica started). Internally, `Forks` consists of multiple layers:
   - `LevelledForest`: A blockchain forms a Tree. When removing all blocks with views strictly smaller than the last finalized block, the chain decomposes into multiple disconnected trees. In graph theory, such structure is a forest. To separate general graph-theoretical concepts from the concrete block-chain application, `LevelledForest` refers to blocks as graph `vertices` and to a block's view number as `level`. The `LevelledForest` is an in-memory data structure to store and maintain a levelled forest. It provides functions to add vertices, query vertices by their ID (block's hash), query vertices by level, query the children of a vertex, and prune vertices by level (remove them from memory).
   - `Finalizer`: the finalizer uses the `LevelledForest` to store blocks. The Finalizer tracks the locked and finalized blocks. Furthermore, it evaluates whether a block is safe to vote for.
   - `ForkChoice`: implements the fork choice rule. Currently, `NewestForkChoice` always uses the quorum certificate (QC) with the largest view to build a new block (i.e. the fork a super-majority voted for most recently).
   - `Forks` is the highest level. It bundles the `Finalizer` and `ForkChoice` into one component.
* `Validator` validates the HotStuff-relevant aspects of
   - QC: total weight of all signers is more than 2/3 of committee weight, validity of signatures, view number is strictly monotonously increasing
   - block proposal: from designated primary for the block's respective view, contains proposer's vote for its own block, QC in block is valid
   - vote: validity of signature, voter is has positive weight 
* `VoteAggregator` caches votes on a per-block basis and builds QC if enough votes have been accumulated.
* `Voter` tracks the view of the latest vote and determines whether or not to vote for a block (by calling `forks.IsSafeBlock`)
* `Committee` maintains the list of all authorized network members and their respective weight on a per-block basis. Furthermore, the committee contains the primary selection algorithm. 
* `BlockProducer` constructs the payload of a block, after the HotStuff core logic has decided which fork to extend 

# Implementation

We have translated the Chained HotStuff protocol into a state machine shown below. The state machine is implemented 
in `EventHandler`.

![](/docs/StateMachine.png)

#### PaceMaker
The HotStuff state machine interacts with the PaceMaker, which triggers view changes. Conceptually, the PaceMaker
interfaces with the `EventHandler` in two different modes:
* [asynchronous] On timeouts, the PaceMaker will emit a timeout event, which is processed as any other event (such as incoming blocks or votes) through the `EventLoop`.
* [synchronous] When progress is made following the happy-path business logic, the  `EventHandler` will inform the PaceMaker about completing the respective 
processing step via a direct method call (see `PaceMaker interface`). If the PaceMaker changed the view in response, it returns 
a `NewViewEvent` which will be synchronously processed by the  `EventHandler`.

Flow's PaceMaker is simple (compared to Libra's PaceMaker for instance). It solely relies on increasing the timeouts 
exponentially if no progress is made and exponentially decreasing timeouts on the happy path. 
The timeout values are limited by lower and upper-bounds to ensure that the PaceMaker can change from large to small timeouts in a reasonable number of views. 
The specific values for lower and upper timeout bounds are protocol-specified; we envision the bounds to be on the order of 1sec (lower bound) and multiple days (upper bound).

**Progress**, from the perspective of the PaceMaker is defined as entering view `V`
for which the replica knows a QC with `V = QC.view + 1`. 
In other words, we transition into the next view due to reaching quorum in the last view.
  
A central, non-trivial functionality of the PaceMaker is to _skip views_. 
Specifically, given a QC with view `qc.view`, the Pacemaker will skip ahead to view `qc.view + 1` if `currentView ≤ qc.view`.
  
<img src="https://github.com/dapperlabs/flow-go/blob/79f48bf9fe067778a19f89f6b9b5ee03972b6c78/docs/PaceMaker.png" width="200">
 
 

## Code structure

All relevant code implementing the core HotStuff business logic is contained in `/consensus/hotstuff/` (folder containing this README).
When starting to look into the code, we suggest starting with `/consensus/hotstuff/event_loop.go` and `/consensus/hotstuff/event_handler.go`. 


### Folder structure

All files in the `/consensus/hotstuff/` folder, except for `event_loop.go` and `follower_loop.go`, are interfaces for HotStuff-related components. 
The concrete implementations for all HotStuff-relevant components are in corresponding sub-folders. 
For completeness, we list the component implemented in each subfolder below: 

* `/consensus/hotstuff/blockproducer` builds a block proposal for a specified QC, interfaces with the logic for assembling a block payload, combines all relevant fields into a new block proposal.
* `/consensus/hotstuff/committee` maintains the list of all authorized network members and their respective weight on a per-block basis; contains the primary selection algorithm. 
* `/consensus/hotstuff/eventhandler` orchestrates all HotStuff components and implements the HotStuff state machine. The event handler is designed to be executed single-threaded.
* `/consensus/hotstuff/follower` This component is only used by nodes that are _not_ participating in the HotStuff committee. As Flow has dedicated node roles with specialized network functions, only a subset of nodes run the full HotStuff protocol. Nevertheless, all nodes need to be able to act on blocks being finalized. The approach we have taken for Flow is that block proposals are broadcast to all nodes (including non-committee nodes). Non-committee nodes locally determine block finality by applying HotStuff's finality rules. The HotStuff Follower contains the functionality to consume block proposals and trigger downstream processing of finalized blocks. The Follower does not _actively_ participate in HotStuff. 
* `/consensus/hotstuff/forks` maintains an in-memory representation of all blocks `b`, whose view is larger or equal to the view of the latest finalized block (known to this specific HotStuff replica). 
It tracks the last finalized block, the currently locked block, evaluates whether it is safe to vote for a given block and provides a Fork-Choice when the replica is primary.  
* `/consensus/hotstuff/helper` contains broadly-used helper functions for testing  
* `/consensus/hotstuff/integration` integration tests for verifying correct interaction of multiple HotStuff replicas
* `/consensus/hotstuff/model` contains the HotStuff data models, including block proposal, QC, etc. 
Many HotStuff data models are built on top of basic data models defined in `/model/flow/`.
* `/consensus/hotstuff/notifications`: All relevant events within the HotStuff logic are exported though a notification system. While the notifications are _not_ used HotStuff-internally, they notify other components within the same node of relevant progress and are used for collecting HotStuff metrics.
* `/consensus/hotstuff/pacemaker` contains the implementation of Flow's basic PaceMaker, as described above.
* `/consensus/hotstuff/persister` for performance reasons, the implementation maintains the consensus state largely in-memory. The `persister` stores the last entered view and the view of the latest voted block persistenlty on disk. This allows recovery after a crash without the risk of equivocation.       
* `/consensus/hotstuff/runner` helper code for starting and shutting down the HotStuff logic safely in a multithreaded environment.  
* `/consensus/hotstuff/validator` holds the logic for validating the HotStuff-relevant aspects of blocks, QCs, and votes
* `/consensus/hotstuff/verification` contains integration of Flow's cryptographic primitives (signing and signature verification) 
* `/consensus/hotstuff/voteaggregator` caches votes on a per-block basis and builds a QC if enough votes have been accumulated.
* `/consensus/hotstuff/voter` tracks the view of the latest vote and determines whether or not to vote for a block


## Pending Extensions  
* BLS Aggregation of the `StakingSignatures`
* include Epochs 
* upgrade PaceMaker to include Timeout Certificates 
* refactor crypto integration (code in `verification` and dependent modules) for better auditability

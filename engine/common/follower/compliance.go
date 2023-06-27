package follower

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// complianceCore interface describes the follower's compliance core logic. Slightly simplified, the
// compliance layer ingests incoming untrusted blocks from the network, filters out all invalid blocks,
// extends the protocol state with the valid blocks, and lastly pipes the valid blocks to the HotStuff
// follower. Conceptually, the algorithm proceeds as follows:
//
//  1. _light_ validation of the block header:
//     - check that the block's proposer is the legitimate leader for the respective view
//     - verify the leader's signature
//     - verify QC within the block
//     - verify whether TC should be included and check the TC
//
//     Optimization for fast catchup:
//     Honest nodes that we synchronize blocks from supply those blocks in sequentially connected order.
//     This allows us to only validate the highest QC of such a sequence. A QC proves validity of the
//     referenced block as well as all its ancestors. The only other detail we have to verify is that the
//     block hashes match with the ParentID in their respective child.
//     To utilize this optimization, we require that the input `connectedRange` is continuous sequence
//     of blocks, i.e. connectedRange[i] is the parent of connectedRange[i+1].
//
//  2. All blocks that pass the light validation go into a size-limited cache with random ejection policy.
//     Under happy operations this cache should not run full, as we prune it by finalized view.
//
//  3. Only certified blocks pass the cache [Note: this is the reason why we need to validate the QC].
//     This caching strategy provides the fist line of defence:
//     - Broken blocks from malicious leaders do not pass this cache, as they will never get certified.
//     - Hardening [heuristic] against spam via block synchronization:
//     TODO: implement
//     We differentiate between two scenarios: (i) the blocks are _all_ already known, i.e. a no-op from
//     the cache's perspective vs (ii) there were some previously unknown blocks in the batch. If and only
//     if there is new information (case ii), we pass the certified blocks to step 4. In case of (i),
//     this is completely redundant information (idempotent), and hence we just exit early.
//     Thereby, the only way for a spamming node to load our higher-level logic is to include
//     valid pending yet previously unknown blocks (very few generally exist in the system).
//
//  4. All certified blocks are passed to the PendingTree, which constructs a graph of all blocks
//     with view greater than the latest finalized block [Note: graph-theoretically this is a forest].
//
//  5. In a nutshell, the PendingTree tracks which blocks have already been connected to the latest finalized
//     block. When adding certified blocks to the PendingTree, it detects additional blocks now connecting
//     the latest finalized block. More formally, the PendingTree locally tracks the tree of blocks rooted
//     on the latest finalized block. When new vertices (i.e. certified blocks) are added to the tree, they
//     they move onto step 6. Blocks are entering step 6 are guaranteed to be in 'parent-first order', i.e.
//     connect to already known blocks. Disconnected blocks remain in the PendingTree, until they are pruned
//     by latest finalized view.
//
//  6. All blocks entering this step are guaranteed to be valid (as they are confirmed to be certified in
//     step 3). Furthermore, we know they connect to previously processed blocks.
//
// On the one hand, step 1 includes CPU-intensive cryptographic checks. On the other hand, it is very well
// parallelizable. In comparison, step 2 and 3 are negligible. Therefore, we can have multiple worker
// routines: a worker takes a batch of transactions and runs it through steps 1,2,3. The blocks that come
// out of step 3, are queued in a channel for further processing.
//
// The PendingTree(step 4) requires very little CPU. Step 5 is a data base write populating many indices,
// to extend the protocol state. Step 6 is only a queuing operation, with vanishing cost. There is little
// benefit to parallelizing state extension, because under normal operations forks are rare and knowing
// the full ancestry is required for the protocol state. Therefore, we have a single thread to extend
// the protocol state with new certified blocks.
//
// Notes:
//   - At the moment, this interface exists to facilitate testing. Specifically, it allows to
//     test the ComplianceEngine with a mock of complianceCore. Higher level business logic does not
//     interact with complianceCore, because complianceCore is wrapped inside the ComplianceEngine.
//   - At the moment, we utilize this interface to also document the algorithmic design.
type complianceCore interface {
	module.Startable
	module.ReadyDoneAware

	// OnBlockRange consumes an *untrusted* range of connected blocks( part of a fork). The originID parameter
	// identifies the node that sent the batch of blocks. The input `connectedRange` must be sequentially ordered
	// blocks that form a chain, i.e. connectedRange[i] is the parent of connectedRange[i+1]. Submitting a
	// disconnected batch results in an `ErrDisconnectedBatch` error and the batch is dropped (no-op).
	// Implementors need to ensure that this function is safe to be used in concurrent environment.
	// Caution: this method is allowed to block.
	// Expected errors during normal operations:
	//   - cache.ErrDisconnectedBatch
	OnBlockRange(originID flow.Identifier, connectedRange []*flow.Block) error

	// OnFinalizedBlock prunes all blocks below the finalized view from the compliance layer's Cache
	// and PendingTree.
	// Caution: this method is allowed to block
	// Implementors need to ensure that this function is safe to be used in concurrent environment.
	OnFinalizedBlock(finalized *flow.Header)
}

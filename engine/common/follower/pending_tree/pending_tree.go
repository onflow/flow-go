package pending_tree

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/forest"
	"github.com/onflow/flow-go/module/mempool"
)

// PendingBlockVertex wraps a block proposal to implement forest.Vertex
// so the proposal can be stored in forest.LevelledForest
type PendingBlockVertex struct {
	flow.CertifiedBlock
	connectedToFinalized bool
}

var _ forest.Vertex = (*PendingBlockVertex)(nil)

// NewVertex creates new vertex while performing a sanity check of data correctness.
func NewVertex(certifiedBlock flow.CertifiedBlock, connectedToFinalized bool) (*PendingBlockVertex, error) {
	return &PendingBlockVertex{
		CertifiedBlock:       certifiedBlock,
		connectedToFinalized: connectedToFinalized,
	}, nil
}

func (v *PendingBlockVertex) VertexID() flow.Identifier { return v.CertifyingQC.BlockID }
func (v *PendingBlockVertex) Level() uint64             { return v.CertifyingQC.View }
func (v *PendingBlockVertex) Parent() (flow.Identifier, uint64) {
	return v.Block.Header.ParentID, v.Block.Header.ParentView
}

// PendingTree is a mempool holding certified blocks that eventually might be connected to the finalized state.
// As soon as a valid fork of certified blocks descending from the latest finalized block is observed,
// we pass this information to caller. Internally, the mempool utilizes the LevelledForest.
// PendingTree is NOT safe to use in concurrent environment.
// Note:
//   - The ability to skip ahead is irrelevant for staked nodes, which continuously follow the chain.
//     However, light clients don't necessarily follow the chain block by block. Assume a light client
//     that knows the EpochCommit event, i.e. the consensus committee authorized to certify blocks. A
//     staked node can easily ship a proof of finalization for a block within that epoch to such a
//     light client. This would be much cheaper for the light client than downloading the headers for
//     all blocks in the epoch.
//   - The pending tree supports skipping ahead, as this is a more general and simpler algorithm.
//     Removing the ability to skip ahead would restrict the PendingTree's domain of potential
//     applications _and_ would require additional code and additional tests making it more complex.
//
// Outlook:
//   - At the moment, PendingTree relies on notion of a `Certified Block` which is a valid block accompanied
//     by a certifying QC (proving block validity). This works well for consensus follower, as it is designed
//     to work with certified blocks.
//   - In the future, we could use the PendingTree also for consensus participants. Therefore, we would need
//     to abstract out CertifiedBlock or replace it with a generic argument that satisfies some contract
//     (returns View, Height, BlockID). Then, consensus participants could use the Pending Tree without
//     QCs and instead fully validate inbound blocks (incl. payloads) to guarantee block validity.
type PendingTree struct {
	forest          *forest.LevelledForest
	lastFinalizedID flow.Identifier
}

// NewPendingTree creates new instance of PendingTree. Accepts finalized block to set up initial state.
func NewPendingTree(finalized *flow.Header) *PendingTree {
	return &PendingTree{
		forest:          forest.NewLevelledForest(finalized.View),
		lastFinalizedID: finalized.ID(),
	}
}

// AddBlocks accepts a batch of certified blocks, adds them to the tree of pending blocks and finds blocks connected to
// the finalized state.
//
// Details:
// Adding blocks might result in additional blocks now being connected to the latest finalized block. The returned
// slice contains:
//  1. the subset of `certifiedBlocks` that are connected to the finalized block
//     - excluding any blocks whose view is smaller or equal to the finalized block
//     - if a block `B ∈ certifiedBlocks` is already known to the PendingTree and connected,
//     `B` and all its connected descendants will be in the returned list
//  2. additionally, all of the _connected_ descendants of the blocks from step 1.
//
// PendingTree treats its input as a potentially repetitive stream of information: repeated inputs are already
// consistent with the current state. While repetitive inputs might cause repetitive outputs, the implementation
// has some general heuristics to avoid extra work:
//   - It drops blocks whose view is smaller or equal to the finalized block
//   - It deduplicates incoming blocks. We don't store additional vertices in tree if we have that block already stored.
//
// Expected errors during normal operations:
//   - model.ByzantineThresholdExceededError - detected two certified blocks at the same view
//
// All other errors should be treated as exceptions.
func (t *PendingTree) AddBlocks(certifiedBlocks []flow.CertifiedBlock) ([]flow.CertifiedBlock, error) {
	var allConnectedBlocks []flow.CertifiedBlock
	for _, block := range certifiedBlocks {
		// skip blocks lower than finalized view
		if block.View() <= t.forest.LowestLevel {
			continue
		}

		iter := t.forest.GetVerticesAtLevel(block.View())
		if iter.HasNext() {
			v := iter.NextVertex().(*PendingBlockVertex)

			if v.VertexID() == block.ID() {
				// this vertex is already in tree, skip it
				continue
			} else {
				return nil, model.ByzantineThresholdExceededError{Evidence: fmt.Sprintf(
					"conflicting QCs at view %d: %v and %v",
					block.View(), v.ID(), block.ID(),
				)}
			}
		}

		vertex, err := NewVertex(block, false)
		if err != nil {
			return nil, fmt.Errorf("could not create new vertex: %w", err)
		}
		err = t.forest.VerifyVertex(vertex)
		if err != nil {
			return nil, fmt.Errorf("failed to store certified block into the tree: %w", err)
		}
		t.forest.AddVertex(vertex)

		if t.connectsToFinalizedBlock(block) {
			allConnectedBlocks = t.updateAndCollectFork(allConnectedBlocks, vertex)
		}
	}

	return allConnectedBlocks, nil
}

// connectsToFinalizedBlock checks if candidate block connects to the finalized state.
func (t *PendingTree) connectsToFinalizedBlock(block flow.CertifiedBlock) bool {
	if block.Block.Header.ParentID == t.lastFinalizedID {
		return true
	}
	if parentVertex, found := t.forest.GetVertex(block.Block.Header.ParentID); found {
		return parentVertex.(*PendingBlockVertex).connectedToFinalized
	}
	return false
}

// FinalizeFork takes last finalized block and prunes all blocks below the finalized view.
// PendingTree treats its input as a potentially repetitive stream of information: repeated
// and older inputs (out of order) are already consistent with the current state. Repetitive
// inputs might cause repetitive outputs.
// When a block is finalized we don't care for any blocks below it, since they were already finalized.
// Finalizing a block might causes the pending PendingTree to detect _additional_ blocks as now
// being connected to the latest finalized block. This happens if some connecting blocks are missing
// and then a block higher than the missing blocks is finalized.
// In the following example, B is the last finalized block known to the PendingTree
//
//	A ← B  ←-?-?-?-- X ← Y ← Z
//
// The network has already progressed to finalizing block X. However, the interim blocks denoted
// by '←-?-?-?--' have not been received by our PendingTree. Therefore, we still consider X,Y,Z
// as disconnected. If the PendingTree tree is now informed that X is finalized, it can fast-
// forward to the respective state, as it anyway would prune all the blocks below X.
// Note:
//   - The ability to skip ahead is irrelevant for staked nodes, which continuously follows the chain.
//     However, light clients don't necessarily follow the chain block by block. Assume a light client
//     that knows the EpochCommit event, i.e. the consensus committee authorized to certify blocks. A
//     staked node can easily ship a proof of finalization for a block within that epoch to such a
//     light client. This would be much cheaper for the light client than downloading the headers for
//     all blocks in the epoch.
//   - The pending tree supports skipping ahead, as this is a more general and simpler algorithm.
//     Removing the ability to skip ahead would restrict the PendingTree's its domain of potential
//     applications _and_ would require additional code and additional tests making it more complex.
//
// If the PendingTree detects additional blocks as descending from the latest finalized block, it
// returns these blocks. Returned blocks are ordered such that parents appear before their children.
//
// No errors are expected during normal operation.
func (t *PendingTree) FinalizeFork(finalized *flow.Header) ([]flow.CertifiedBlock, error) {
	var connectedBlocks []flow.CertifiedBlock

	err := t.forest.PruneUpToLevel(finalized.View)
	if err != nil {
		if mempool.IsBelowPrunedThresholdError(err) {
			return nil, nil
		}
		return connectedBlocks, fmt.Errorf("could not prune tree up to view %d: %w", finalized.View, err)
	}
	t.lastFinalizedID = finalized.ID()

	iter := t.forest.GetChildren(t.lastFinalizedID)
	for iter.HasNext() {
		v := iter.NextVertex().(*PendingBlockVertex)
		connectedBlocks = t.updateAndCollectFork(connectedBlocks, v)
	}

	return connectedBlocks, nil
}

// updateAndCollectFork marks the subtree rooted at `vertex.Block` as connected to the finalized state
// and returns all blocks in this subtree. No parents of `vertex.Block` are modified or included in the output.
// The output list will be ordered so that parents appear before children.
// The caller must ensure that `vertex.Block` is connected to the finalized state.
//
//	A ← B ← C ←D
//	      ↖ E
//
// For example, suppose B is the input vertex. Then:
//   - A must already be connected to the finalized state
//   - B, E, C, D are marked as connected to the finalized state and included in the output list
//
// This method has a similar signature as `append` for performance reasons:
//   - any connected certified blocks are appended to `queue`
//   - we return the _resulting slice_ after all appends
func (t *PendingTree) updateAndCollectFork(queue []flow.CertifiedBlock, vertex *PendingBlockVertex) []flow.CertifiedBlock {
	if vertex.connectedToFinalized {
		return queue // no-op if already connected
	}
	vertex.connectedToFinalized = true
	queue = append(queue, vertex.CertifiedBlock)

	iter := t.forest.GetChildren(vertex.VertexID())
	for iter.HasNext() {
		nextVertex := iter.NextVertex().(*PendingBlockVertex)
		queue = t.updateAndCollectFork(queue, nextVertex)
	}
	return queue
}

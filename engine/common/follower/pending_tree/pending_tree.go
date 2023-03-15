package pending_tree

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/forest"
)

// CertifiedBlock holds a certified block, it consists of a block and a QC which proves validity of block (QC.BlockID = Block.ID())
// This is used to compactly store and transport block and certifying QC in one structure.
type CertifiedBlock struct {
	Block *flow.Block
	QC    *flow.QuorumCertificate
}

// ID returns unique identifier for the certified block
// To avoid computation we use value from the QC
func (b *CertifiedBlock) ID() flow.Identifier {
	return b.QC.BlockID
}

// View returns view where the block was produced.
func (b *CertifiedBlock) View() uint64 {
	return b.QC.View
}

// Height returns height of the block.
func (b *CertifiedBlock) Height() uint64 {
	return b.Block.Header.Height
}

// PendingBlockVertex wraps a block proposal to implement forest.Vertex
// so the proposal can be stored in forest.LevelledForest
type PendingBlockVertex struct {
	CertifiedBlock
	connectedToFinalized bool
}

var _ forest.Vertex = (*PendingBlockVertex)(nil)

// NewVertex creates new vertex while performing a sanity check of data correctness.
func NewVertex(certifiedBlock CertifiedBlock, connectedToFinalized bool) (*PendingBlockVertex, error) {
	if certifiedBlock.Block.Header.View != certifiedBlock.QC.View {
		return nil, fmt.Errorf("missmatched block(%d) and QC(%d) view",
			certifiedBlock.Block.Header.View, certifiedBlock.QC.View)
	}
	return &PendingBlockVertex{
		CertifiedBlock:       certifiedBlock,
		connectedToFinalized: connectedToFinalized,
	}, nil
}

func (v *PendingBlockVertex) VertexID() flow.Identifier { return v.QC.BlockID }
func (v *PendingBlockVertex) Level() uint64             { return v.QC.View }
func (v *PendingBlockVertex) Parent() (flow.Identifier, uint64) {
	return v.Block.Header.ParentID, v.Block.Header.ParentView
}

// PendingTree is a mempool holding certified blocks that eventually might be connected to the finalized state.
// As soon as a valid fork of certified blocks descending from the latest finalized block is observed,
// we pass this information to caller. Internally, the mempool utilizes the LevelledForest.
// PendingTree is NOT safe to use in concurrent environment.
// NOTE: PendingTree relies on notion of `CertifiedBlock` which is a valid block accompanied by a certifying QC (proving block validity).
// This works well for consensus follower as it is designed to work with certified blocks. To use this structure for consensus
// participant we can abstract out CertifiedBlock or replace it with a generic argument that satisfies some contract(returns View, Height, BlockID).
// With this change this structure can be used by consensus participant for tracking connection to the finalized state even without
// having QC but relying on payload validation.
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

// AddBlocks accepts a batch of certified blocks, adds them to the tree of pending blocks and finds blocks connected to the finalized state.
// This function performs processing of incoming certified blocks, implementation is split into a few different sections
// but tries to be optimal in terms of performance to avoid doing extra work as much as possible.
// This function proceeds as follows:
//  1. Sorts incoming batch by height. Since blocks can be submitted in random order we need to find blocks with
//     the lowest height since they are candidates for being connected to the finalized state.
//  2. Filters out blocks that are already finalized.
//  3. Deduplicates incoming blocks. We don't store additional vertices in tree if we have that block already stored.
//  4. Checks for exceeding byzantine threshold. Only one certified block per view is allowed.
//  5. Finally, blocks with the lowest height from incoming batch that connect to the finalized state we will
//     mark all descendants as connected, collect them and return as result of invocation.
//
// This function is designed to perform resolution of connected blocks(resolved block is the one that connects to the finalized state)
// using incoming batch. Each block that was connected to the finalized state is reported once.
// Expected errors during normal operations:
//   - model.ByzantineThresholdExceededError - detected two certified blocks at the same view
func (t *PendingTree) AddBlocks(certifiedBlocks []CertifiedBlock) ([]CertifiedBlock, error) {
	var allConnectedBlocks []CertifiedBlock
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
func (t *PendingTree) connectsToFinalizedBlock(block CertifiedBlock) bool {
	if block.Block.Header.ParentID == t.lastFinalizedID {
		return true
	}
	if parentVertex, found := t.forest.GetVertex(block.Block.Header.ParentID); found {
		return parentVertex.(*PendingBlockVertex).connectedToFinalized
	}
	return false
}

// FinalizeForkAtLevel takes last finalized block and prunes levels below the finalized view.
// When a block is finalized we don't care for all blocks below it since they were already finalized.
// Finalizing a block might result in observing a connected chain of blocks that previously weren't.
// These blocks will be returned as result of invocation.
// No errors are expected during normal operation.
func (t *PendingTree) FinalizeForkAtLevel(finalized *flow.Header) ([]CertifiedBlock, error) {
	var connectedBlocks []CertifiedBlock
	blockID := finalized.ID()
	if t.forest.LowestLevel >= finalized.View {
		return connectedBlocks, nil
	}

	t.lastFinalizedID = blockID
	err := t.forest.PruneUpToLevel(finalized.View)
	if err != nil {
		return connectedBlocks, fmt.Errorf("could not prune tree up to view %d: %w", finalized.View, err)
	}

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
func (t *PendingTree) updateAndCollectFork(queue []CertifiedBlock, vertex *PendingBlockVertex) []CertifiedBlock {
	vertex.connectedToFinalized = true
	queue = append(queue, vertex.CertifiedBlock)

	iter := t.forest.GetChildren(vertex.VertexID())
	for iter.HasNext() {
		nextVertex := iter.NextVertex().(*PendingBlockVertex)
		// if it's already connected then it was already reported
		if !nextVertex.connectedToFinalized {
			queue = t.updateAndCollectFork(queue, nextVertex)
		}
	}
	return queue
}

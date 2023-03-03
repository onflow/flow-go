package pending_tree

import (
	"fmt"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/forest"
	"sync"
)

// CertifiedBlock holds a certified block, it consists of block itself and a QC which proofs validity of block.
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

// NewVertex creates new vertex while performing a sanity check of data correctness
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
// As soon as a valid fork of certified blocks descending from the latest finalized block we pass this information to caller.
// Internally, the mempool utilizes the LevelledForest.
type PendingTree struct {
	forest          *forest.LevelledForest
	lock            sync.RWMutex
	lastFinalizedID flow.Identifier
}

func NewPendingTree(finalized *flow.Header) *PendingTree {
	return &PendingTree{
		forest:          forest.NewLevelledForest(finalized.View),
		lastFinalizedID: finalized.ID(),
	}
}

// AddBlocks accepts a batch of certified blocks, adds them to the tree of pending blocks and finds blocks connected to the finalized state.
// This function performs processing of incoming certified blocks, implementation is split into a few different sections
// but tries to be optimal in terms of performance to avoid doing extra work as much as possible.
// This function follows next implementation:
//  1. Filters out blocks that are already finalized.
//  2. Finds block with the lowest height. Since blocks can be submitted in random order we need to find block with
//     the lowest height since it's the candidate for being connected to the finalized state.
//  3. Deduplicates incoming blocks. We don't store additional vertices in tree if we have that block already stored.
//  4. Checks for exceeding byzantine threshold. Only one certified block per view is allowed.
//  5. Finally, block with the lowest height from incoming batch connects to the finalized state we will
//     mark all descendants as connected, collect them and return as result of invocation.
//
// This function is designed to collect all connected blocks to the finalized state if lowest block(by height) connects to it.
// Expected errors during normal operations:
//   - model.ByzantineThresholdExceededError - detected two certified blocks at the same view
func (t *PendingTree) AddBlocks(certifiedBlocks []CertifiedBlock) ([]CertifiedBlock, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	var connectedBlocks []CertifiedBlock
	firstBlockIndex := -1
	for i, block := range certifiedBlocks {
		// skip blocks lower than finalized view
		if block.View() <= t.forest.LowestLevel {
			continue
		}

		// We need to find the lowest block by height since it has the possibility to be connected to finalized block.
		// We can't use view here, since when chain forks we might have view > height.
		if firstBlockIndex < 0 || certifiedBlocks[firstBlockIndex].Height() > block.Height() {
			firstBlockIndex = i
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
	}

	// all blocks were below finalized height, and we have nothing to do.
	if firstBlockIndex < 0 {
		return nil, nil
	}

	var connectedToFinalized bool
	firstBlock := certifiedBlocks[firstBlockIndex]
	if firstBlock.Block.Header.ParentID == t.lastFinalizedID {
		connectedToFinalized = true
	} else if parentVertex, found := t.forest.GetVertex(firstBlock.Block.Header.ParentID); found {
		connectedToFinalized = parentVertex.(*PendingBlockVertex).connectedToFinalized
	}

	if connectedToFinalized {
		vertex, _ := t.forest.GetVertex(firstBlock.ID())
		connectedBlocks = t.updateAndCollectFork(vertex.(*PendingBlockVertex))
	}

	return connectedBlocks, nil
}

func (t *PendingTree) FinalizeForkAtLevel(finalized *flow.Header) error {
	blockID := finalized.ID()
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.forest.LowestLevel >= finalized.View {
		return nil
	}

	t.lastFinalizedID = blockID
	err := t.forest.PruneUpToLevel(finalized.View)
	if err != nil {
		return fmt.Errorf("could not prune tree up to view %d: %w", finalized.View, err)
	}
	return nil
}

func (t *PendingTree) updateAndCollectFork(vertex *PendingBlockVertex) []CertifiedBlock {
	certifiedBlocks := []CertifiedBlock{vertex.CertifiedBlock}
	vertex.connectedToFinalized = true
	iter := t.forest.GetChildren(vertex.VertexID())
	for iter.HasNext() {
		blocks := t.updateAndCollectFork(iter.NextVertex().(*PendingBlockVertex))
		certifiedBlocks = append(certifiedBlocks, blocks...)
	}
	return certifiedBlocks
}

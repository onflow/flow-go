package pending_tree

import (
	"fmt"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/forest"
	"sync"
)

type CertifiedBlock struct {
	Block *flow.Block
	QC    *flow.QuorumCertificate
}

func (b *CertifiedBlock) ID() flow.Identifier {
	return b.QC.BlockID
}

func (b *CertifiedBlock) View() uint64 {
	return b.QC.View
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

// AddBlocks accepts a batch of certified blocks in ascending height order.
// Skips in height between blocks are allowed.
// Expected errors during normal operations:
//   - model.ByzantineThresholdExceededError - detected two certified blocks at the same view
func (t *PendingTree) AddBlocks(certifiedBlocks []CertifiedBlock) ([]CertifiedBlock, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	var connectedBlocks []CertifiedBlock
	for _, block := range certifiedBlocks {
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

	firstBlock := certifiedBlocks[0]

	var connectedToFinalized bool
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

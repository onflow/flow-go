package pending_tree

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/forest"
	"sync"
)

type CertifiedBlock struct {
	Block *flow.Block
	QC    *flow.QuorumCertificate
}

// PendingBlockVertex wraps a block proposal to implement forest.Vertex
// so the proposal can be stored in forest.LevelledForest
type PendingBlockVertex struct {
	block                *flow.Block
	qc                   *flow.QuorumCertificate
	connectedToFinalized bool
}

func (v *PendingBlockVertex) VertexID() flow.Identifier { return v.qc.BlockID }
func (v *PendingBlockVertex) Level() uint64             { return v.qc.View }
func (v *PendingBlockVertex) Parent() (flow.Identifier, uint64) {
	return v.block.Header.ParentID, v.block.Header.ParentView
}

// PendingTree is a mempool holding certified blocks that eventually might be connected to the finalized state.
// As soon as a valid fork of certified blocks descending from the latest finalized block we pass this information to caller.
// Internally, the mempool utilizes the LevelledForest.
type PendingTree struct {
	forest            *forest.LevelledForest
	lock              sync.RWMutex
	lastFinalizedID   flow.Identifier
	lastFinalizedView uint64
}

func NewPendingTree() *PendingTree {
	return &PendingTree{}
}

func (t *PendingTree) AddBlocks(certifiedBlocks []*flow.Block, certifyingQC *flow.QuorumCertificate) ([]CertifiedBlock, error) {
	qcs := make([]*flow.QuorumCertificate, 0, len(certifiedBlocks))
	for _, block := range certifiedBlocks[1:] {
		qcs = append(qcs, block.Header.QuorumCertificate())
	}
	qcs = append(qcs, certifyingQC)

	t.lock.Lock()

	parentVertex, found := t.forest.GetVertex(certifiedBlocks[0].Header.ParentID)
	var connectedToFinalized bool
	if found {
		connectedToFinalized = parentVertex.(*PendingBlockVertex).connectedToFinalized
	}

	var connectedBlocks []CertifiedBlock
	for i, block := range certifiedBlocks {
		iter := t.forest.GetVerticesAtLevel(block.Header.View)
		if iter.HasNext() {
			v := iter.NextVertex()
			if v.VertexID() == block.ID() {
				// this vertex is already in tree, skip it
				continue
			} else {
				// TODO: raise this properly
				panic("protocol violation, two certified blocks at same height, byzantine threshold exceeded")
			}
		}

		vertex := &PendingBlockVertex{
			block:                block,
			qc:                   qcs[i],
			connectedToFinalized: connectedToFinalized,
		}
		err := t.forest.VerifyVertex(vertex)
		if err != nil {
			return nil, fmt.Errorf("failed to store certified block into the tree: %w", err)
		}
		t.forest.AddVertex(vertex)

		if connectedToFinalized && i == len(certifiedBlocks)-1 {
			connectedBlocks = t.updateAndCollectFork(vertex)
		}
	}
	t.lock.Unlock()
	return connectedBlocks, nil
}

func (t *PendingTree) updateAndCollectFork(vertex *PendingBlockVertex) []CertifiedBlock {
	certifiedBlocks := []CertifiedBlock{{
		Block: vertex.block,
		QC:    vertex.qc,
	}}
	vertex.connectedToFinalized = true
	iter := t.forest.GetChildren(vertex.VertexID())
	for iter.HasNext() {
		blocks := t.updateAndCollectFork(iter.NextVertex().(*PendingBlockVertex))
		certifiedBlocks = append(certifiedBlocks, blocks...)
	}
	return certifiedBlocks
}

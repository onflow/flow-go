package pending_tree

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/forest"
	"sync"
)

// PendingBlockVertex wraps a block proposal to implement forest.Vertex
// so the proposal can be stored in forest.LevelledForest
type PendingBlockVertex struct {
	block *flow.Block
	qc    *flow.QuorumCertificate
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
	lastFinalizedView uint64
}

func NewPendingTree() *PendingTree {
	return &PendingTree{}
}

func AddBlocks(certifiedBlocks []*flow.Block, certifyingQC *flow.QuorumCertificate) (*[]flow.Block, *flow.QuorumCertificate) {

}

package buffer

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/forest"
)

// proposalVertex implements [forest.Vertex] for generic block proposals.
//
//structwrite:immutable
type proposalVertex[T module.BufferedProposal] struct {
	proposal flow.Slashable[T]
	id       flow.Identifier
}

// header is a shortform way to access the proposal's header.
func (v proposalVertex[T]) header() *flow.Header {
	return v.proposal.Message.ProposalHeader().Header
}

func newProposalVertex[T module.BufferedProposal](proposal flow.Slashable[T]) proposalVertex[T] {
	return proposalVertex[T]{
		proposal: proposal,
		id:       proposal.Message.ProposalHeader().Header.ID(),
	}
}

// VertexID returns the block ID for the stored proposal.
func (v proposalVertex[T]) VertexID() flow.Identifier {
	return v.id
}

// Level returns the view for the stored proposal.
func (v proposalVertex[T]) Level() uint64 {
	return v.header().View
}

// Parent returns the parent ID and view for the stored proposal.
func (v proposalVertex[T]) Parent() (flow.Identifier, uint64) {
	return v.header().ParentID, v.header().ParentView
}

// GenericPendingBlocks implements a mempool of pending blocks that cannot yet be processed
// because they do not connect to the rest of the chain state.
// They are indexed by parent ID to enable processing all of a parent's children once the parent is received.
// They are also indexed by view to support pruning.
// The size of this mempool is partly limited by the enforcement of an allowed view range, however
// a strong size limit also requires that stored proposals are validated to ensure we store only
// one proposal per view. Higher-level logic is responsible for validating proposals prior to storing here.
//
// Safe for concurrent use.
type GenericPendingBlocks[T module.BufferedProposal] struct {
	// TODO concurrency
	forest *forest.LevelledForest
}

type PendingBlocks = GenericPendingBlocks[*flow.Proposal]
type PendingClusterBlocks = GenericPendingBlocks[*cluster.Proposal]

var _ module.PendingBlockBuffer = (*PendingBlocks)(nil)
var _ module.PendingClusterBlockBuffer = (*PendingClusterBlocks)(nil)

// TODO: inject finalizedView
func NewPendingBlocks() *PendingBlocks {
	return &PendingBlocks{forest: forest.NewLevelledForest(1_000_000)}
}

func NewPendingClusterBlocks() *PendingClusterBlocks {
	return &PendingClusterBlocks{forest: forest.NewLevelledForest(1_000_000)}
}

func (b *GenericPendingBlocks[T]) Add(block flow.Slashable[T]) {
	b.forest.AddVertex(newProposalVertex(block))
}

func (b *GenericPendingBlocks[T]) ByID(blockID flow.Identifier) (flow.Slashable[T], bool) {
	vertex, ok := b.forest.GetVertex(blockID)
	if !ok {
		return flow.Slashable[T]{}, false
	}
	return vertex.(proposalVertex[T]).proposal, true
}

func (b *GenericPendingBlocks[T]) ByParentID(parentID flow.Identifier) ([]flow.Slashable[T], bool) {
	n := b.forest.GetNumberOfChildren(parentID)
	if n == 0 {
		return nil, false
	}

	children := make([]flow.Slashable[T], 0, n)
	iterator := b.forest.GetChildren(parentID)
	for iterator.HasNext() {
		vertex := iterator.NextVertex()
		children = append(children, vertex.(proposalVertex[T]).proposal)
	}

	return children, true
}

// TODO: remove this
func (b *GenericPendingBlocks[T]) DropForParent(parentID flow.Identifier) {
	//b.forest.
	//b.backend.dropForParent(parentID)
}

// PruneByView prunes any pending cluster blocks with views less or equal to the given view.
func (b *GenericPendingBlocks[T]) PruneByView(view uint64) {
	err := b.forest.PruneUpToLevel(view - 1) // TODO: OBO here
	_ = err                                  // TODO: deal with error here
}

func (b *GenericPendingBlocks[T]) Size() uint {
	return uint(b.forest.GetSize())
}

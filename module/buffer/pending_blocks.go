package buffer

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/forest"
)

// proposalVertex
// TODO: docs
//
//structwrite:immutable
type proposalVertex[T module.BufferedProposal] struct {
	proposal flow.Slashable[T]
	id       flow.Identifier
}

func (v proposalVertex[T]) header() *flow.Header {
	return v.proposal.Message.ProposalHeader().Header
}

func newProposalVertex[T module.BufferedProposal](proposal flow.Slashable[T]) proposalVertex[T] {
	return proposalVertex[T]{
		proposal: proposal,
		id:       proposal.Message.ProposalHeader().Header.ID(),
	}
}

func (v proposalVertex[T]) VertexID() flow.Identifier {
	return v.id
}

func (v proposalVertex[T]) Level() uint64 {
	return v.header().View
}

func (v proposalVertex[T]) Parent() (flow.Identifier, uint64) {
	return v.header().ParentID, v.header().ParentView
}

type GenericPendingBlocks[T module.BufferedProposal] struct {
	forest *forest.LevelledForest
}

type PendingClusterBlocks = GenericPendingBlocks[*cluster.Proposal]
type PendingBlocks = GenericPendingBlocks[*flow.Proposal]

func NewPendingClusterBlocks() *PendingClusterBlocks {
	return &PendingClusterBlocks{forest: forest.NewLevelledForest(1_000_000)}
}

func NewPendingBlocks() *PendingBlocks {
	return &PendingBlocks{forest: forest.NewLevelledForest(1_000_000)}
}

// TODO remove bool return here
func (b *GenericPendingBlocks[T]) Add(block flow.Slashable[T]) bool {
	b.forest.AddVertex(newProposalVertex(block))
	return true
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

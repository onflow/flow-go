package buffer

import (
	"sync"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/forest"
	"github.com/onflow/flow-go/module/mempool"
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
//
// Safe for concurrent use.
type GenericPendingBlocks[T module.BufferedProposal] struct {
	lock                *sync.Mutex
	forest              *forest.LevelledForest
	activeViewRangeSize uint64
}

type PendingBlocks = GenericPendingBlocks[*flow.Proposal]
type PendingClusterBlocks = GenericPendingBlocks[*cluster.Proposal]

var _ module.PendingBlockBuffer = (*PendingBlocks)(nil)
var _ module.PendingClusterBlockBuffer = (*PendingClusterBlocks)(nil)

func NewPendingBlocks(finalizedView uint64, activeViewRangeSize uint64) *PendingBlocks {
	return &PendingBlocks{
		lock: new(sync.Mutex),
		// LevelledForest's lowestLevel is inclusive, so add 1 here
		forest:              forest.NewLevelledForest(finalizedView + 1),
		activeViewRangeSize: activeViewRangeSize,
	}
}

func NewPendingClusterBlocks(finalizedView uint64, activeViewRangeSize uint64) *PendingClusterBlocks {
	return &PendingClusterBlocks{
		lock:                new(sync.Mutex),
		forest:              forest.NewLevelledForest(finalizedView),
		activeViewRangeSize: activeViewRangeSize,
	}
}

// Add adds the input block to the block buffer.
// If the block already exists, or is below the finalized view, this is a no-op.
// Errors returns:
//   - mempool.BeyondActiveRangeError if block.View > finalizedView + activeViewRangeSize (when activeViewRangeSize > 0)
func (b *GenericPendingBlocks[T]) Add(block flow.Slashable[T]) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	blockView := block.Message.ProposalHeader().Header.View
	finalizedView := b.highestPrunedView()

	// Check if block view exceeds the active view range size
	// If activeViewRangeSize is 0, there's no limitation
	if b.activeViewRangeSize > 0 && blockView > finalizedView+b.activeViewRangeSize {
		return mempool.NewBeyondActiveRangeError(
			"block view %d exceeds active view range size: finalized view %d + range size %d = %d",
			blockView, finalizedView, b.activeViewRangeSize, finalizedView+b.activeViewRangeSize,
		)
	}

	b.forest.AddVertex(newProposalVertex(block))
	return nil
}

// ByID returns the block with the given ID, if it exists.
// Otherwise returns (nil, false)
func (b *GenericPendingBlocks[T]) ByID(blockID flow.Identifier) (flow.Slashable[T], bool) {
	b.lock.Lock()
	defer b.lock.Unlock()
	vertex, ok := b.forest.GetVertex(blockID)
	if !ok {
		return flow.Slashable[T]{}, false
	}
	return vertex.(proposalVertex[T]).proposal, true
}

// ByParentID returns all direct children of the given block.
// If no children with the given parent exist, returns (nil, false)
func (b *GenericPendingBlocks[T]) ByParentID(parentID flow.Identifier) ([]flow.Slashable[T], bool) {
	b.lock.Lock()
	defer b.lock.Unlock()
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

// PruneByView prunes all pending blocks with views less or equal to the given view.
// Errors returns:
//   - mempool.BelowPrunedThresholdError if input level is below the lowest retained view (finalized view)
func (b *GenericPendingBlocks[T]) PruneByView(view uint64) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	// PruneUpToLevel prunes up to be EXCLUDING the input view, so add 1 here
	return b.forest.PruneUpToLevel(view + 1)
}

// Size returns the number of blocks in the buffer.
func (b *GenericPendingBlocks[T]) Size() uint {
	b.lock.Lock()
	defer b.lock.Unlock()
	return uint(b.forest.GetSize())
}

// highestPrunedView returns the highest pruned view (finalized view).
// CAUTION: Caller must acquire the lock.
func (b *GenericPendingBlocks[T]) highestPrunedView() uint64 {
	// LevelledForest.LowestLevel is the lowest UNPRUNED view, so subtract 1 here
	return b.forest.LowestLevel - 1
}

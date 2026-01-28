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
type proposalVertex[T flow.HashablePayload] struct {
	proposal flow.Slashable[*flow.GenericProposal[T]]
	id       flow.Identifier
}

func newProposalVertex[T flow.HashablePayload](proposal flow.Slashable[*flow.GenericProposal[T]]) proposalVertex[T] {
	return proposalVertex[T]{
		proposal: proposal,
		id:       proposal.Message.Block.ID(),
	}
}

// VertexID returns the block ID for the stored proposal.
func (v proposalVertex[T]) VertexID() flow.Identifier {
	return v.id
}

// Level returns the view for the stored proposal.
func (v proposalVertex[T]) Level() uint64 {
	return v.proposal.Message.Block.View
}

// Parent returns the parent ID and view for the stored proposal.
func (v proposalVertex[T]) Parent() (flow.Identifier, uint64) {
	return v.proposal.Message.Block.ParentID, v.proposal.Message.Block.ParentView
}

// GenericPendingBlocks implements a mempool of pending blocks that cannot yet be processed
// because they do not connect to the rest of the chain state.
// Under the hood, we utilize the LevelledForest to store the pending blocks:
//   - The LevelledForest automatically indexes all vertices by their parent ID. This allows us
//     to process all of a parent's children once the parent is received.
//   - The LevelledForest automatically indexes all vertices by their level (for which we use
//     the view here) and provides efficient pruning of all vertices below a certain level / view.
//
// The primary use case of this buffer is to cache blocks until they can be processed once their
// ancestry is known. However, care must be taken to avoid unbounded memory growth which can be
// exploited by byzantine proposers to mount memory exhaustion attacks.
//
// In order to mitigate memory exhaustion attacks ActiveViewRangeSize to the PendingBlockBuffer,
// which puts an upper limit on what views we will accept when adding blocks. However, this only
// limits the depth of the tree, but not thw tree's width. A byzantine proposer might create
// lots of conflicting blocks (valid or invalid). We prevent unbounded memory growth from such
// attacks by limiting how many proposals are stored per one view. For this particular implementation we
// are storing one proposal per view. Strictly speaking there could be multiple valid proposals for single view
// from single proposer, and we don't know which one will be certified. We rely on syncing of certified blocks
// to guarantee liveness of the system. If by accident a node has cached a proposal which won't be certified it will
// eventually receive it as a certified block from the sync engine.
//
// Safe for concurrent use.
type GenericPendingBlocks[T flow.HashablePayload] struct {
	lock                *sync.Mutex
	forest              *forest.LevelledForest
	activeViewRangeSize uint64
}

type PendingBlocks = GenericPendingBlocks[flow.Payload]
type PendingClusterBlocks = GenericPendingBlocks[cluster.Payload]

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
		lock: new(sync.Mutex),
		// LevelledForest's lowestLevel is inclusive, so add 1 here
		forest:              forest.NewLevelledForest(finalizedView + 1),
		activeViewRangeSize: activeViewRangeSize,
	}
}

// Add adds the input block to the block buffer.
// If the block already exists, is below the finalized view, or another block for this view has been already stored this is a no-op.
// Errors returns:
//   - mempool.BeyondActiveRangeError if block.View > finalizedView + activeViewRangeSize (when activeViewRangeSize > 0)
func (b *GenericPendingBlocks[T]) Add(block flow.Slashable[*flow.GenericProposal[T]]) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	blockView := block.Message.Block.View
	finalizedView := b.highestPrunedView()

	// Check if block view exceeds the active view range size
	// If activeViewRangeSize is 0, there's no limitation
	if b.activeViewRangeSize > 0 && blockView > finalizedView+b.activeViewRangeSize {
		return mempool.NewBeyondActiveRangeError(
			"block view %d exceeds active view range size: finalized view %d + range size %d = %d",
			blockView, finalizedView, b.activeViewRangeSize, finalizedView+b.activeViewRangeSize,
		)
	}

	// to ensure that we store one proposal per view, we are adding a vertex iff there is no vertex at the given level,
	// this guarantees that we store single proposal per view.
	if b.forest.GetNumberOfVerticesAtLevel(blockView) == 0 {
		b.forest.AddVertex(newProposalVertex(block))
	}
	return nil
}

// ByID returns the block with the given ID, if it exists, otherwise returns (nil, false).
func (b *GenericPendingBlocks[T]) ByID(blockID flow.Identifier) (flow.Slashable[*flow.GenericProposal[T]], bool) {
	b.lock.Lock()
	defer b.lock.Unlock()
	vertex, ok := b.forest.GetVertex(blockID)
	if !ok {
		return flow.Slashable[*flow.GenericProposal[T]]{}, false
	}
	return vertex.(proposalVertex[T]).proposal, true
}

// ByView returns all stored blocks with the given view.
// If none are found an empty array is returned
func (b *GenericPendingBlocks[T]) ByView(view uint64) []flow.Slashable[*flow.GenericProposal[T]] {
	b.lock.Lock()
	defer b.lock.Unlock()
	iter := b.forest.GetVerticesAtLevel(view)
	var result []flow.Slashable[*flow.GenericProposal[T]]
	for iter.HasNext() {
		result = append(result, iter.NextVertex().(proposalVertex[T]).proposal)
	}
	return result
}

// ByParentID returns all direct children of the given block.
// If no children with the given parent exist, returns (nil, false)
func (b *GenericPendingBlocks[T]) ByParentID(parentID flow.Identifier) ([]flow.Slashable[*flow.GenericProposal[T]], bool) {
	b.lock.Lock()
	defer b.lock.Unlock()
	n := b.forest.GetNumberOfChildren(parentID)
	if n == 0 {
		return nil, false
	}

	children := make([]flow.Slashable[*flow.GenericProposal[T]], 0, n)
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

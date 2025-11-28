package buffer

import (
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/utils/unittest"
)

type PendingBlocksSuite struct {
	suite.Suite
	buffer *GenericPendingBlocks[*flow.Proposal]
}

func TestPendingBlocksSuite(t *testing.T) {
	suite.Run(t, new(PendingBlocksSuite))
}

func (suite *PendingBlocksSuite) SetupTest() {
	// Initialize with finalized view 0 and no view range limitation (0 = no limit)
	// Individual tests that need the limitation will create their own buffers
	suite.buffer = NewPendingBlocks(0, 0)
}

// block creates a new block proposal wrapped as Slashable.
func (suite *PendingBlocksSuite) block() flow.Slashable[*flow.Proposal] {
	block := unittest.BlockFixture()
	return unittest.AsSlashable(unittest.ProposalFromBlock(block))
}

// blockWithParent creates a new block proposal with the given parent header.
func (suite *PendingBlocksSuite) blockWithParent(parent *flow.Header) flow.Slashable[*flow.Proposal] {
	block := unittest.BlockWithParentFixture(parent)
	return unittest.AsSlashable(unittest.ProposalFromBlock(block))
}

// TestAdd tests adding blocks to the buffer.
func (suite *PendingBlocksSuite) TestAdd() {
	block := suite.block()
	suite.Require().NoError(suite.buffer.Add(block))

	// Verify block can be retrieved by ID
	retrieved, ok := suite.buffer.ByID(block.Message.Block.ID())
	suite.Assert().True(ok)
	suite.Assert().Equal(block.Message.Block.ID(), retrieved.Message.Block.ID())
	suite.Assert().Equal(block.Message.Block.View, retrieved.Message.Block.View)

	// Verify block can be retrieved by parent ID
	children, ok := suite.buffer.ByParentID(block.Message.Block.ParentID)
	suite.Assert().True(ok)
	suite.Assert().Len(children, 1)
	suite.Assert().Equal(block.Message.Block.ID(), children[0].Message.Block.ID())
}

// TestAddDuplicate verifies that adding the same block twice is a no-op.
func (suite *PendingBlocksSuite) TestAddDuplicate() {
	block := suite.block()
	suite.Require().NoError(suite.buffer.Add(block))
	suite.Require().NoError(suite.buffer.Add(block)) // Add again

	// Should still only have one block
	suite.Assert().Equal(uint(1), suite.buffer.Size())

	// Should still be retrievable
	retrieved, ok := suite.buffer.ByID(block.Message.Block.ID())
	suite.Assert().True(ok)
	suite.Assert().Equal(block.Message.Block.ID(), retrieved.Message.Block.ID())
}

// TestAddBelowFinalizedView verifies that adding blocks below finalized view is a no-op.
func (suite *PendingBlocksSuite) TestAddBelowFinalizedView() {
	finalizedView := uint64(1000)
	buffer := NewPendingBlocks(finalizedView, 100_000)

	// Create a block with view below finalized
	block := suite.block()
	block.Message.Block.ParentView = finalizedView - 10
	block.Message.Block.View = finalizedView - 5

	suite.Require().NoError(buffer.Add(block))

	_, ok := buffer.ByID(block.Message.Block.ID())
	suite.Assert().False(ok)
	suite.Assert().Equal(uint(0), buffer.Size())
}

// TestAddExceedsActiveViewRangeSize verifies that adding blocks that exceed the active view range size returns an error.
func (suite *PendingBlocksSuite) TestAddExceedsActiveViewRangeSize() {
	finalizedView := uint64(1000)
	activeViewRangeSize := uint64(100)
	buffer := NewPendingBlocks(finalizedView, activeViewRangeSize)

	// Create a parent header and then a block that exceeds the active view range size
	parentHeader := unittest.BlockHeaderFixture()
	parentHeader.View = finalizedView + 50
	block := suite.blockWithParent(parentHeader)
	block.Message.Block.View = finalizedView + activeViewRangeSize + 1

	err := buffer.Add(block)
	suite.Assert().Error(err)
	suite.Assert().True(mempool.IsBeyondActiveRangeError(err))

	// Verify block was not added
	_, ok := buffer.ByID(block.Message.Block.ID())
	suite.Assert().False(ok)
	suite.Assert().Equal(uint(0), buffer.Size())
}

// TestAddWithinActiveViewRangeSize verifies that adding blocks within the active view range size succeeds.
func (suite *PendingBlocksSuite) TestAddWithinActiveViewRangeSize() {
	finalizedView := uint64(1000)
	activeViewRangeSize := uint64(100)
	buffer := NewPendingBlocks(finalizedView, activeViewRangeSize)

	// Create a parent header and then a block that is exactly at the limit
	parentHeader := unittest.BlockHeaderFixture()
	parentHeader.View = finalizedView + 50
	block := suite.blockWithParent(parentHeader)
	block.Message.Block.View = finalizedView + activeViewRangeSize

	err := buffer.Add(block)
	suite.Assert().NoError(err)

	// Verify block was added
	_, ok := buffer.ByID(block.Message.Block.ID())
	suite.Assert().True(ok)
	suite.Assert().Equal(uint(1), buffer.Size())
}

// TestAddWithZeroActiveViewRangeSize verifies that when activeViewRangeSize is 0, there's no limitation.
func (suite *PendingBlocksSuite) TestAddWithZeroActiveViewRangeSize() {
	finalizedView := uint64(1000)
	activeViewRangeSize := uint64(0) // No limitation
	buffer := NewPendingBlocks(finalizedView, activeViewRangeSize)

	// Create a parent header and then a block that is very far ahead
	parentHeader := unittest.BlockHeaderFixture()
	parentHeader.View = finalizedView + 500_000
	block := suite.blockWithParent(parentHeader)
	block.Message.Block.View = finalizedView + 1_000_000

	err := buffer.Add(block)
	suite.Assert().NoError(err)

	// Verify block was added
	_, ok := buffer.ByID(block.Message.Block.ID())
	suite.Assert().True(ok)
	suite.Assert().Equal(uint(1), buffer.Size())
}

// TestByID tests retrieving blocks by ID.
func (suite *PendingBlocksSuite) TestByID() {
	block := suite.block()
	suite.Require().NoError(suite.buffer.Add(block))

	// Test retrieving existing block
	retrieved, ok := suite.buffer.ByID(block.Message.Block.ID())
	suite.Assert().True(ok)
	suite.Assert().Equal(block.Message.Block.ID(), retrieved.Message.Block.ID())

	// Test retrieving non-existent block
	nonExistentID := unittest.IdentifierFixture()
	_, ok = suite.buffer.ByID(nonExistentID)
	suite.Assert().False(ok)
}

// TestByParentID tests retrieving blocks by parent ID.
func (suite *PendingBlocksSuite) TestByParentID() {
	parent := suite.block()
	suite.buffer.Add(parent)

	// Create multiple children of the parent
	child1 := suite.blockWithParent(parent.Message.Block.ToHeader())
	child2 := suite.blockWithParent(parent.Message.Block.ToHeader())
	grandchild := suite.blockWithParent(child1.Message.Block.ToHeader())
	unrelated := suite.block()

	suite.buffer.Add(child1)
	suite.buffer.Add(child2)
	suite.buffer.Add(grandchild)
	suite.buffer.Add(unrelated)

	// Test retrieving children of parent
	children, ok := suite.buffer.ByParentID(parent.Message.Block.ID())
	suite.Assert().True(ok)
	suite.Assert().Len(children, 2)

	// Verify correct children are returned
	retrievedChildIDs := make(map[flow.Identifier]bool)
	for _, child := range children {
		retrievedChildIDs[child.Message.Block.ID()] = true
	}
	suite.Assert().True(retrievedChildIDs[child1.Message.Block.ID()])
	suite.Assert().True(retrievedChildIDs[child2.Message.Block.ID()])

	// Test retrieving children of non-existent parent
	nonExistentParentID := unittest.IdentifierFixture()
	_, ok = suite.buffer.ByParentID(nonExistentParentID)
	suite.Assert().False(ok)
}

// TestByParentIDOnlyDirectChildren verifies that ByParentID only returns direct children.
func (suite *PendingBlocksSuite) TestByParentIDOnlyDirectChildren() {
	parent := suite.block()
	suite.buffer.Add(parent)

	child := suite.blockWithParent(parent.Message.Block.ToHeader())
	grandchild := suite.blockWithParent(child.Message.Block.ToHeader())

	suite.buffer.Add(child)
	suite.buffer.Add(grandchild)

	// Parent should only have child, not grandchild
	children, ok := suite.buffer.ByParentID(parent.Message.Block.ID())
	suite.Assert().True(ok)
	suite.Assert().Len(children, 1)
	suite.Assert().Equal(child.Message.Block.ID(), children[0].Message.Block.ID())

	// Child should have grandchild
	grandchildren, ok := suite.buffer.ByParentID(child.Message.Block.ID())
	suite.Assert().True(ok)
	suite.Assert().Len(grandchildren, 1)
	suite.Assert().Equal(grandchild.Message.Block.ID(), grandchildren[0].Message.Block.ID())
}

// TestPruneByView tests pruning blocks by view.
func (suite *PendingBlocksSuite) TestPruneByView() {
	const N = 100 // number of blocks to test with
	blocks := make([]flow.Slashable[*flow.Proposal], 0, N)

	// Build a buffer with various blocks
	for i := 0; i < N; i++ {
		// 10% of the time, add a new unrelated block
		if i%10 == 0 {
			block := suite.block()
			suite.Require().NoError(suite.buffer.Add(block))
			blocks = append(blocks, block)
			continue
		}

		// 90% of the time, build on an existing block
		if i%2 == 1 && len(blocks) > 0 {
			parent := blocks[rand.Intn(len(blocks))]
			block := suite.blockWithParent(parent.Message.Block.ToHeader())
			suite.Require().NoError(suite.buffer.Add(block))
			blocks = append(blocks, block)
		}
	}

	// Pick a view to prune that's guaranteed to prune at least one block
	pruneAt := blocks[rand.Intn(len(blocks))].Message.Block.View
	err := suite.buffer.PruneByView(pruneAt)
	suite.Assert().NoError(err)

	// Verify blocks at or below prune view are removed
	for _, block := range blocks {
		view := block.Message.Block.View
		id := block.Message.Block.ID()

		if view <= pruneAt {
			_, exists := suite.buffer.ByID(id)
			suite.Assert().False(exists, "block at view %d should be pruned", view)
		} else {
			_, exists := suite.buffer.ByID(id)
			suite.Assert().True(exists, "block at view %d should not be pruned", view)
		}
	}
}

// TestPruneByViewBelowFinalizedView verifies that pruning below finalized view returns an error.
func (suite *PendingBlocksSuite) TestPruneByViewBelowFinalizedView() {
	finalizedView := uint64(100)
	buffer := NewPendingBlocks(finalizedView, 100_000)

	// Add some blocks above finalized view
	parent := unittest.BlockHeaderFixture()
	parent.View = finalizedView + 10
	block := suite.blockWithParent(parent)
	suite.Require().NoError(buffer.Add(block))

	// Prune at finalized view should succeed
	err := buffer.PruneByView(finalizedView)
	suite.Assert().NoError(err)

	// Prune below finalized view should fail
	err = buffer.PruneByView(finalizedView - 1)
	suite.Assert().Error(err)
	suite.Assert().True(mempool.IsBelowPrunedThresholdError(err))
}

// TestPruneByViewMultipleTimes tests that pruning multiple times works correctly.
func (suite *PendingBlocksSuite) TestPruneByViewMultipleTimes() {
	// Create blocks at different views
	parentHeader := unittest.BlockHeaderFixture()
	parentHeader.View = 10
	parent := suite.blockWithParent(parentHeader)
	suite.buffer.Add(parent)

	// Create children - views will be automatically set to be greater than parent
	child1 := suite.blockWithParent(parent.Message.Block.ToHeader())
	child1.Message.Block.View++
	suite.buffer.Add(child1)

	child2 := suite.blockWithParent(parent.Message.Block.ToHeader())
	suite.buffer.Add(child2)

	// Get actual views
	parentView := parent.Message.Block.View
	child1View := child1.Message.Block.View
	child2View := child2.Message.Block.View

	// Prune at the parent's view (should remove parent only)
	pruneView1 := parentView
	err := suite.buffer.PruneByView(pruneView1)
	suite.Assert().NoError(err)

	// Verify parent is pruned but children remain
	suite.Assert().Equal(uint(2), suite.buffer.Size())
	_, ok := suite.buffer.ByID(parent.Message.Block.ID())
	suite.Assert().False(ok)
	_, ok = suite.buffer.ByID(child1.Message.Block.ID())
	suite.Assert().True(ok)
	_, ok = suite.buffer.ByID(child2.Message.Block.ID())
	suite.Assert().True(ok)

	// Prune at a view that removes all remaining blocks
	pruneView2 := max(child1View, child2View)

	err = suite.buffer.PruneByView(pruneView2)
	suite.Assert().NoError(err)
	suite.Assert().Equal(uint(0), suite.buffer.Size())
}

// TestSize tests the Size method.
func (suite *PendingBlocksSuite) TestSize() {
	// Initially empty
	suite.Assert().Equal(uint(0), suite.buffer.Size())

	// Add blocks and verify size increases
	block1 := suite.block()
	suite.buffer.Add(block1)
	suite.Assert().Equal(uint(1), suite.buffer.Size())

	block2 := suite.block()
	suite.buffer.Add(block2)
	suite.Assert().Equal(uint(2), suite.buffer.Size())

	// Adding duplicate should not increase size
	suite.buffer.Add(block1)
	suite.Assert().Equal(uint(2), suite.buffer.Size())
}

// TestConcurrentAccess tests that the buffer is safe for concurrent access.
// NOTE: correctness here depends on [PendingBlockSuite.block] not returning duplicates.
func (suite *PendingBlocksSuite) TestConcurrentAccess() {
	const numGoroutines = 10
	const blocksPerGoroutine = 10

	wg := new(sync.WaitGroup)
	wg.Add(numGoroutines)

	// Concurrently add blocks
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < blocksPerGoroutine; j++ {
				block := suite.block()
				suite.Require().NoError(suite.buffer.Add(block))
			}
		}()
	}
	wg.Wait()

	// Verify all blocks were added
	suite.Assert().Equal(uint(numGoroutines*blocksPerGoroutine), suite.buffer.Size())
}

// TestEmptyBufferOperations tests operations on an empty buffer.
func (suite *PendingBlocksSuite) TestEmptyBufferOperations() {
	suite.Assert().Equal(uint(0), suite.buffer.Size())

	// ByID should return false
	_, ok := suite.buffer.ByID(unittest.IdentifierFixture())
	suite.Assert().False(ok)

	// ByParentID should return false
	_, ok = suite.buffer.ByParentID(unittest.IdentifierFixture())
	suite.Assert().False(ok)

	// PruneByView should succeed (no-op)
	err := suite.buffer.PruneByView(100)
	suite.Assert().NoError(err)
}

// TestAddAfterPrune verifies that blocks can be added after pruning.
func (suite *PendingBlocksSuite) TestAddAfterPrune() {
	// Add and prune a block
	block1 := suite.block()
	suite.buffer.Add(block1)

	err := suite.buffer.PruneByView(block1.Message.Block.View)
	suite.Assert().NoError(err)

	// Verify block is pruned
	_, ok := suite.buffer.ByID(block1.Message.Block.ID())
	suite.Assert().False(ok)

	// Add a new block after pruning with view above pruned view
	block2 := suite.blockWithParent(block1.Message.Block.ToHeader())
	suite.buffer.Add(block2)

	// Verify new block is added
	retrieved, ok := suite.buffer.ByID(block2.Message.Block.ID())
	suite.Assert().True(ok)
	suite.Assert().Equal(block2.Message.Block.ID(), retrieved.Message.Block.ID())
	suite.Assert().Equal(uint(1), suite.buffer.Size())
}

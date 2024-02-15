package pending_tree

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/exp/slices"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestPendingTree(t *testing.T) {
	suite.Run(t, new(PendingTreeSuite))
}

type PendingTreeSuite struct {
	suite.Suite

	finalized   *flow.Header
	pendingTree *PendingTree
}

func (s *PendingTreeSuite) SetupTest() {
	s.finalized = unittest.BlockHeaderFixture()
	s.pendingTree = NewPendingTree(s.finalized)
}

// TestBlocksConnectToFinalized tests that adding blocks that directly connect to the finalized block result
// in expect chain of connected blocks.
// Having: F ← B1 ← B2 ← B3
// Add [B1, B2, B3], expect to get [B1;QC_B1, B2;QC_B2; B3;QC_B3]
func (s *PendingTreeSuite) TestBlocksConnectToFinalized() {
	blocks := certifiedBlocksFixture(3, s.finalized)
	connectedBlocks, err := s.pendingTree.AddBlocks(blocks)
	require.NoError(s.T(), err)
	require.Equal(s.T(), blocks, connectedBlocks)
}

// TestBlocksAreNotConnectedToFinalized tests that adding blocks that don't connect to the finalized block result
// in empty list of connected blocks.
// Having: F ← B1 ← B2 ← B3
// Add [B2, B3], expect to get []
func (s *PendingTreeSuite) TestBlocksAreNotConnectedToFinalized() {
	blocks := certifiedBlocksFixture(3, s.finalized)
	connectedBlocks, err := s.pendingTree.AddBlocks(blocks[1:])
	require.NoError(s.T(), err)
	require.Empty(s.T(), connectedBlocks)
}

// TestInsertingMissingBlockToFinalized tests that adding blocks that don't connect to the finalized block result
// in empty list of connected blocks. After adding missing blocks all connected blocks are correctly returned.
// Having: F ← B1 ← B2 ← B3 ← B4 ← B5
// Add [B3, B4, B5], expect to get []
// Add [B1, B2], expect to get [B1, B2, B3, B4, B5]
func (s *PendingTreeSuite) TestInsertingMissingBlockToFinalized() {
	blocks := certifiedBlocksFixture(5, s.finalized)
	connectedBlocks, err := s.pendingTree.AddBlocks(blocks[len(blocks)-3:])
	require.NoError(s.T(), err)
	require.Empty(s.T(), connectedBlocks)

	connectedBlocks, err = s.pendingTree.AddBlocks(blocks[:len(blocks)-3])
	require.NoError(s.T(), err)
	require.Equal(s.T(), blocks, connectedBlocks)
}

// TestInsertingMissingBlockToFinalized tests that adding blocks that don't connect to the finalized block result
// in empty list of connected blocks. After adding missing block all connected blocks across all forks are correctly collected
// and returned.
// Having:
//
//	       ↙ B2 ← B3
//	F ← B1 ← B4 ← B5 ← B6 ← B7
//
// Add [B2, B3], expect to get []
// Add [B4, B5, B6, B7], expect to get []
// Add [B1], expect to get [B1, B2, B3, B4, B5, B6, B7]
func (s *PendingTreeSuite) TestAllConnectedForksAreCollected() {
	longestFork := certifiedBlocksFixture(5, s.finalized)
	B2 := unittest.BlockWithParentFixture(longestFork[0].Block.Header)
	// make sure short fork doesn't have conflicting views, so we don't trigger exception
	B2.Header.View = longestFork[len(longestFork)-1].Block.Header.View + 1
	B3 := unittest.BlockWithParentFixture(B2.Header)
	shortFork := []flow.CertifiedBlock{{
		Block:        B2,
		CertifyingQC: B3.Header.QuorumCertificate(),
	}, certifiedBlockFixture(B3)}

	connectedBlocks, err := s.pendingTree.AddBlocks(shortFork)
	require.NoError(s.T(), err)
	require.Empty(s.T(), connectedBlocks)

	connectedBlocks, err = s.pendingTree.AddBlocks(longestFork[1:])
	require.NoError(s.T(), err)
	require.Empty(s.T(), connectedBlocks)

	connectedBlocks, err = s.pendingTree.AddBlocks(longestFork[:1])
	require.NoError(s.T(), err)
	require.ElementsMatch(s.T(), append(longestFork, shortFork...), connectedBlocks)
}

// TestAddingConnectedBlocks tests that adding blocks that were already reported as connected is no-op.
func (s *PendingTreeSuite) TestAddingConnectedBlocks() {
	blocks := certifiedBlocksFixture(3, s.finalized)
	connectedBlocks, err := s.pendingTree.AddBlocks(blocks)
	require.NoError(s.T(), err)
	require.Equal(s.T(), blocks, connectedBlocks)

	connectedBlocks, err = s.pendingTree.AddBlocks(blocks)
	require.NoError(s.T(), err)
	require.Empty(s.T(), connectedBlocks)
}

// TestByzantineThresholdExceeded tests that submitting two certified blocks for the same view is reported as
// byzantine threshold reached exception. This scenario is possible only if network has reached more than 1/3 byzantine participants.
func (s *PendingTreeSuite) TestByzantineThresholdExceeded() {
	block := unittest.BlockWithParentFixture(s.finalized)
	conflictingBlock := unittest.BlockWithParentFixture(s.finalized)
	// use same view for conflicted blocks, this is not possible unless there is more than
	// 1/3 byzantine participants
	conflictingBlock.Header.View = block.Header.View
	_, err := s.pendingTree.AddBlocks([]flow.CertifiedBlock{certifiedBlockFixture(block)})
	require.NoError(s.T(), err)
	// adding same block should result in no-op
	_, err = s.pendingTree.AddBlocks([]flow.CertifiedBlock{certifiedBlockFixture(block)})
	require.NoError(s.T(), err)
	connectedBlocks, err := s.pendingTree.AddBlocks([]flow.CertifiedBlock{certifiedBlockFixture(conflictingBlock)})
	require.Empty(s.T(), connectedBlocks)
	require.True(s.T(), model.IsByzantineThresholdExceededError(err))
}

// TestBatchWithSkipsAndInRandomOrder tests that providing a batch without specific order and even with skips in height
// results in expected behavior. We expect that each of those blocks will be added to tree and as soon as we find a
// finalized fork we should be able to observe it as result of invocation.
// Having: F ← A ← B ← C ← D ← E
// Randomly shuffle [B, C, D, E] and add it as single batch, expect [] connected blocks.
// Insert [A], expect [A, B, C, D, E] connected blocks.
func (s *PendingTreeSuite) TestBatchWithSkipsAndInRandomOrder() {
	blocks := certifiedBlocksFixture(5, s.finalized)

	rand.Shuffle(len(blocks)-1, func(i, j int) {
		blocks[i+1], blocks[j+1] = blocks[j+1], blocks[i+1]
	})
	connectedBlocks, err := s.pendingTree.AddBlocks(blocks[1:])
	require.NoError(s.T(), err)
	assert.Empty(s.T(), connectedBlocks)

	connectedBlocks, err = s.pendingTree.AddBlocks(blocks[0:1])
	require.NoError(s.T(), err)

	// restore view based order since that's what we will get from PendingTree
	slices.SortFunc(blocks, func(lhs flow.CertifiedBlock, rhs flow.CertifiedBlock) int {
		return int(lhs.View()) - int(rhs.View())
	})

	assert.Equal(s.T(), blocks, connectedBlocks)
}

// TestResolveBlocksAfterFinalization tests that finalizing a block performs resolution against tree state and collects
// newly connected blocks(with the respect to new finalized state) and returns them as result.
// Having:
//
//	       ↙ B2 ← B3
//	F ← B1 ← B4 ← B5 ← B6 ← B7
//
// Add [B2, B3], expect to get []
// Add [B5, B6, B7], expect to get []
// Finalize B4, expect to get [B5, B6, B7]
func (s *PendingTreeSuite) TestResolveBlocksAfterFinalization() {
	longestFork := certifiedBlocksFixture(5, s.finalized)
	B2 := unittest.BlockWithParentFixture(longestFork[0].Block.Header)
	// make sure short fork doesn't have conflicting views, so we don't trigger exception
	B2.Header.View = longestFork[len(longestFork)-1].Block.Header.View + 1
	B3 := unittest.BlockWithParentFixture(B2.Header)
	shortFork := []flow.CertifiedBlock{{
		Block:        B2,
		CertifyingQC: B3.Header.QuorumCertificate(),
	}, certifiedBlockFixture(B3)}

	connectedBlocks, err := s.pendingTree.AddBlocks(shortFork)
	require.NoError(s.T(), err)
	require.Empty(s.T(), connectedBlocks)

	connectedBlocks, err = s.pendingTree.AddBlocks(longestFork[2:])
	require.NoError(s.T(), err)
	require.Empty(s.T(), connectedBlocks)

	connectedBlocks, err = s.pendingTree.FinalizeFork(longestFork[1].Block.Header)
	require.NoError(s.T(), err)
	require.ElementsMatch(s.T(), longestFork[2:], connectedBlocks)
}

// TestBlocksLowerThanFinalizedView tests that implementation drops blocks lower than finalized view.
func (s *PendingTreeSuite) TestBlocksLowerThanFinalizedView() {
	block := unittest.BlockWithParentFixture(s.finalized)
	newFinalized := unittest.BlockWithParentFixture(block.Header)
	_, err := s.pendingTree.FinalizeFork(newFinalized.Header)
	require.NoError(s.T(), err)
	_, err = s.pendingTree.AddBlocks([]flow.CertifiedBlock{certifiedBlockFixture(block)})
	require.NoError(s.T(), err)
	require.Equal(s.T(), uint64(0), s.pendingTree.forest.GetSize())
}

// TestAddingBlockAfterFinalization tests that adding a batch of blocks which includes finalized block correctly returns
// a chain of connected blocks without finalized one.
// Having F ← A ← B ← D.
// Adding [A, B, C] returns [A, B, C].
// Finalize A.
// Adding [A, B, C, D] returns [D] since A is already finalized, [B, C] are already stored and connected to the finalized state.
func (s *PendingTreeSuite) TestAddingBlockAfterFinalization() {
	blocks := certifiedBlocksFixture(4, s.finalized)

	connectedBlocks, err := s.pendingTree.AddBlocks(blocks[:3])
	require.NoError(s.T(), err)
	assert.Equal(s.T(), blocks[:3], connectedBlocks)

	_, err = s.pendingTree.FinalizeFork(blocks[0].Block.Header)
	require.NoError(s.T(), err)

	connectedBlocks, err = s.pendingTree.AddBlocks(blocks)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), blocks[3:], connectedBlocks)
}

// TestAddingBlocksWithSameHeight tests that adding blocks with same height(which results in multiple forks) that are connected
// to finalized state are properly marked and returned as connected blocks.
// / Having F ← A ← C
// /          ↖ B ← D ← E
// Adding [A, B, D] returns [A, B, D]
// Adding [C, E] returns [C, E].
func (s *PendingTreeSuite) TestAddingBlocksWithSameHeight() {
	A := unittest.BlockWithParentFixture(s.finalized)
	B := unittest.BlockWithParentFixture(s.finalized)
	B.Header.View = A.Header.View + 1
	C := unittest.BlockWithParentFixture(A.Header)
	C.Header.View = B.Header.View + 1
	D := unittest.BlockWithParentFixture(B.Header)
	D.Header.View = C.Header.View + 1
	E := unittest.BlockWithParentFixture(D.Header)
	E.Header.View = D.Header.View + 1

	firstBatch := []flow.CertifiedBlock{certifiedBlockFixture(A), certifiedBlockFixture(B), certifiedBlockFixture(D)}
	secondBatch := []flow.CertifiedBlock{certifiedBlockFixture(C), certifiedBlockFixture(E)}

	actual, err := s.pendingTree.AddBlocks(firstBatch)
	require.NoError(s.T(), err)
	require.Equal(s.T(), firstBatch, actual)

	actual, err = s.pendingTree.AddBlocks(secondBatch)
	require.NoError(s.T(), err)
	require.Equal(s.T(), secondBatch, actual)
}

// certifiedBlocksFixture builds a chain of certified blocks starting at some block.
func certifiedBlocksFixture(count int, parent *flow.Header) []flow.CertifiedBlock {
	result := make([]flow.CertifiedBlock, 0, count)
	blocks := unittest.ChainFixtureFrom(count, parent)
	for i := 0; i < count-1; i++ {
		certBlock, err := flow.NewCertifiedBlock(blocks[i], blocks[i+1].Header.QuorumCertificate())
		if err != nil {
			// this should never happen, as we are specifically constructing a certifying QC for the input block
			panic(fmt.Sprintf("unexpected error constructing certified block: %s", err.Error()))
		}
		result = append(result, certBlock)
	}
	result = append(result, certifiedBlockFixture(blocks[len(blocks)-1]))
	return result
}

// certifiedBlockFixture builds a certified block using a QC with fixture signatures.
func certifiedBlockFixture(block *flow.Block) flow.CertifiedBlock {
	certBlock, err := flow.NewCertifiedBlock(block, unittest.CertifyBlock(block.Header))
	if err != nil {
		// this should never happen, as we are specifically constructing a certifying QC for the input block
		panic(fmt.Sprintf("unexpected error constructing certified block: %s", err.Error()))
	}
	return certBlock
}

package pending_tree

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

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
// Having: F <- B1 <- B2 <- B3
// Add [B1, B2, B3], expect to get [B1;QC_B1, B2;QC_B2; B3;QC_B3]
func (s *PendingTreeSuite) TestBlocksConnectToFinalized() {
	blocks := unittest.ChainFixtureFrom(3, s.finalized)
	connectedBlocks, err := s.pendingTree.AddBlocks(blocks, certifyLast(blocks))
	require.NoError(s.T(), err)
	require.Equal(s.T(), blocks, unwrapCertifiedBlocks(connectedBlocks), certifyLast(blocks))
}

// TestBlocksAreNotConnectedToFinalized tests that adding blocks that don't connect to the finalized block result
// in empty list of connected blocks.
// Having: F <- B1 <- B2 <- B3
// Add [B2, B3], expect to get []
func (s *PendingTreeSuite) TestBlocksAreNotConnectedToFinalized() {
	blocks := unittest.ChainFixtureFrom(3, s.finalized)
	connectedBlocks, err := s.pendingTree.AddBlocks(blocks[1:], certifyLast(blocks))
	require.NoError(s.T(), err)
	require.Empty(s.T(), connectedBlocks)
}

// TestInsertingMissingBlockToFinalized tests that adding blocks that don't connect to the finalized block result
// in empty list of connected blocks. After adding missing blocks all connected blocks are correctly returned.
// Having: F <- B1 <- B2 <- B3 <- B4 <- B5
// Add [B3, B4, B5], expect to get []
// Add [B1, B2], expect to get [B1, B2, B3, B4, B5]
func (s *PendingTreeSuite) TestInsertingMissingBlockToFinalized() {
	blocks := unittest.ChainFixtureFrom(5, s.finalized)
	connectedBlocks, err := s.pendingTree.AddBlocks(blocks[len(blocks)-3:], certifyLast(blocks))
	require.NoError(s.T(), err)
	require.Empty(s.T(), connectedBlocks)

	connectedBlocks, err = s.pendingTree.AddBlocks(blocks[:len(blocks)-3], blocks[len(blocks)-3].Header.QuorumCertificate())
	require.NoError(s.T(), err)
	require.Equal(s.T(), blocks, unwrapCertifiedBlocks(connectedBlocks))
}

// TestInsertingMissingBlockToFinalized tests that adding blocks that don't connect to the finalized block result
// in empty list of connected blocks. After adding missing block all connected blocks across all forks are correctly collected
// and returned.
// Having: <- B2 <- B3
// F <- B1 <- B4 <- B5 <- B6 <- B7
// Add [B2, B3], expect to get []
// Add [B4, B5, B6, B7], expect to get []
// Add [B1], expect to get [B1, B2, B3, B4, B5, B6, B7]
func (s *PendingTreeSuite) TestAllConnectedForksAreCollected() {
	longestFork := unittest.ChainFixtureFrom(5, s.finalized)
	B2 := unittest.BlockWithParentFixture(longestFork[0].Header)
	// make sure short fork doesn't have conflicting views, so we don't trigger exception
	B2.Header.View = longestFork[len(longestFork)-1].Header.View + 1
	B3 := unittest.BlockWithParentFixture(B2.Header)
	shortFork := []*flow.Block{B2, B3}

	connectedBlocks, err := s.pendingTree.AddBlocks(shortFork, certifyLast(shortFork))
	require.NoError(s.T(), err)
	require.Empty(s.T(), connectedBlocks)

	connectedBlocks, err = s.pendingTree.AddBlocks(longestFork[1:], certifyLast(longestFork))
	require.NoError(s.T(), err)
	require.Empty(s.T(), connectedBlocks)

	connectedBlocks, err = s.pendingTree.AddBlocks(longestFork[:1], longestFork[1].Header.QuorumCertificate())
	require.NoError(s.T(), err)
	require.ElementsMatch(s.T(), append(longestFork, shortFork...), unwrapCertifiedBlocks(connectedBlocks))
}

func unwrapCertifiedBlocks(certified []CertifiedBlock) []*flow.Block {
	blocks := make([]*flow.Block, 0, len(certified))
	for _, cert := range certified {
		blocks = append(blocks, cert.Block)
	}
	return blocks
}

func certifyLast(blocks []*flow.Block) *flow.QuorumCertificate {
	return unittest.CertifyBlock(blocks[len(blocks)-1].Header)
}

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

func unwrapCertifiedBlocks(certified []CertifiedBlock) []*flow.Block {
	blocks := make([]*flow.Block, 0, len(certified))
	for _, cert := range certified {
		blocks = append(blocks, cert.Block)
	}
	return blocks
}

func (s *PendingTreeSuite) TestBlocksConnectToFinalized() {
	blocks := unittest.ChainFixtureFrom(3, s.finalized)
	connectedBlocks, err := s.pendingTree.AddBlocks(blocks, unittest.CertifyBlock(blocks[len(blocks)-1].Header))
	require.NoError(s.T(), err)
	require.Equal(s.T(), blocks, unwrapCertifiedBlocks(connectedBlocks))
}

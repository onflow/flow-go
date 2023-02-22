package follower

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/common/follower/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCache(t *testing.T) {
	suite.Run(t, new(CacheSuite))
}

type CacheSuite struct {
	suite.Suite

	onEquivocation *mock.OnEquivocation
	cache          *Cache
}

func (s *CacheSuite) SetupTest() {
	collector := metrics.NewNoopCollector()
	s.onEquivocation = mock.NewOnEquivocation(s.T())
	s.cache = NewCache(unittest.Logger(), 1000, collector, s.onEquivocation.Execute)
}

// TestPeek tests if previously added block can be queried by block ID
func (s *CacheSuite) TestPeek() {
	block := unittest.BlockFixture()
	s.cache.AddBlocks([]*flow.Block{&block})
	actual := s.cache.Peek(block.ID())
	require.NotNil(s.T(), actual)
	require.Equal(s.T(), actual.ID(), block.ID())
}

// TestAddBlocksChildCertifiesParent tests a scenario: A <- B[QC_A].
// First we add A and then B, in two different batches.
// We expect that A will get certified after adding B.
func (s *CacheSuite) TestChildCertifiesParent() {
	block := unittest.BlockFixture()
	certifiedBatch, certifyingQC := s.cache.AddBlocks([]*flow.Block{&block})
	require.Empty(s.T(), certifiedBatch)
	require.Nil(s.T(), certifyingQC)
	child := unittest.BlockWithParentFixture(block.Header)
	certifiedBatch, certifyingQC = s.cache.AddBlocks([]*flow.Block{child})
	require.Len(s.T(), certifiedBatch, 1)
	require.NotNil(s.T(), certifyingQC)
	require.Equal(s.T(), block.ID(), certifyingQC.BlockID)
	require.Equal(s.T(), certifiedBatch[0], &block)
}

// TestChildBeforeParent tests a scenario: A <- B[QC_A].
// First we add B and then A, in two different batches.
// We expect that A will get certified after adding A.
func (s *CacheSuite) TestChildBeforeParent() {
	blocks, _, _ := unittest.ChainFixture(2)
	s.cache.AddBlocks([]*flow.Block{blocks[1]})
	certifiedBatch, certifyingQC := s.cache.AddBlocks([]*flow.Block{blocks[0]})
	require.Len(s.T(), certifiedBatch, 1)
	require.NotNil(s.T(), certifyingQC)
	require.Equal(s.T(), blocks[0].ID(), certifyingQC.BlockID)
	require.Equal(s.T(), certifiedBatch[0], blocks[0])
}

// TestAddBatch tests a scenario: B1 <- ... <- BN added in one batch.
// We expect that all blocks except the last one will be certified.
// Certifying QC will be taken from last block.
func (s *CacheSuite) TestAddBatch() {
	blocks, _, _ := unittest.ChainFixture(10)
	certifiedBatch, certifyingQC := s.cache.AddBlocks(blocks)
	require.Equal(s.T(), blocks[:len(blocks)-1], certifiedBatch)
	require.Equal(s.T(), blocks[len(blocks)-1].Header.QuorumCertificate(), certifyingQC)
}

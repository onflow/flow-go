package cache

import (
	"golang.org/x/exp/slices"
	"sync"
	"testing"
	"time"

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

// TestConcurrentAdd simulates multiple workers adding batches of blocks out of order.
// We use next setup:
// Number of workers - workers
// Number of batches submitted by worker - batchesPerWorker
// Number of blocks in each batch submitted by worker - blocksPerBatch
// Each worker submits batchesPerWorker*blocksPerBatch blocks
// In total we will submit workers*batchesPerWorker*blocksPerBatch
// After submitting all blocks we expect that chain of blocks except last one will get certified.
func (s *CacheSuite) TestConcurrentAdd() {
	workers := 5
	batchesPerWorker := 10
	blocksPerBatch := 10
	blocksPerWorker := blocksPerBatch * batchesPerWorker
	// ChainFixture generates N+1 blocks since it adds a root block
	blocks, _, _ := unittest.ChainFixture(workers*blocksPerWorker - 1)
	var wg sync.WaitGroup
	wg.Add(workers)

	var certifiedBlocksLock sync.Mutex
	var allCertifiedBlocks []*flow.Block

	for i := 0; i < workers; i++ {
		go func(blocks []*flow.Block) {
			defer wg.Done()
			for batch := 0; batch < batchesPerWorker; batch++ {
				certifiedBlocks, _ := s.cache.AddBlocks(blocks[batch*blocksPerBatch : (batch+1)*blocksPerBatch])
				certifiedBlocksLock.Lock()
				allCertifiedBlocks = append(allCertifiedBlocks, certifiedBlocks...)
				certifiedBlocksLock.Unlock()
			}
		}(blocks[i*blocksPerWorker : (i+1)*blocksPerWorker])
	}

	unittest.RequireReturnsBefore(s.T(), wg.Wait, time.Millisecond*500, "should submit blocks before timeout")

	require.Len(s.T(), allCertifiedBlocks, len(blocks)-1)
	slices.SortFunc(allCertifiedBlocks, func(lhs *flow.Block, rhs *flow.Block) bool {
		return lhs.Header.Height < rhs.Header.Height
	})
	require.Equal(s.T(), blocks[:len(blocks)-1], allCertifiedBlocks)
}

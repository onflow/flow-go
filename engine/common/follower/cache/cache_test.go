package cache

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"
	"golang.org/x/exp/slices"

	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCache(t *testing.T) {
	suite.Run(t, new(CacheSuite))
}

const defaultHeroCacheLimit = 1000

// CacheSuite holds minimal state for testing Cache in different test scenarios.
type CacheSuite struct {
	suite.Suite

	consumer *mocks.ProposalViolationConsumer
	cache    *Cache
}

func (s *CacheSuite) SetupTest() {
	collector := metrics.NewNoopCollector()
	s.consumer = mocks.NewProposalViolationConsumer(s.T())
	s.cache = NewCache(unittest.Logger(), defaultHeroCacheLimit, collector, s.consumer)
}

// TestPeek tests if previously added blocks can be queried by block ID.
func (s *CacheSuite) TestPeek() {
	blocks := unittest.ProposalChainFixtureFrom(10, unittest.BlockHeaderFixture())
	_, err := s.cache.AddBlocks(blocks)
	require.NoError(s.T(), err)
	for _, proposal := range blocks {
		actual := s.cache.Peek(proposal.Block.Hash())
		require.NotNil(s.T(), actual)
		require.Equal(s.T(), actual.Block.Hash(), proposal.Block.Hash())
	}
}

// TestBlocksEquivocation tests that cache tracks blocks equivocation when adding blocks that have the same view
// but different block ID. Equivocation is a symptom of byzantine actions and needs to be detected and addressed.
func (s *CacheSuite) TestBlocksEquivocation() {
	blocks := unittest.ProposalChainFixtureFrom(10, unittest.BlockHeaderFixture())
	_, err := s.cache.AddBlocks(blocks)
	require.NoError(s.T(), err)
	// adding same blocks again shouldn't result in any equivocation events
	_, err = s.cache.AddBlocks(blocks)
	require.NoError(s.T(), err)

	equivocatedBlocks, _, _ := unittest.ChainFixture(len(blocks) - 1)
	equivocatedProposals := make([]*flow.Proposal, 0, len(equivocatedBlocks)-1)

	// we will skip genesis block as it will be the same
	for i := 1; i < len(equivocatedBlocks); i++ {
		block := equivocatedBlocks[i]
		// update view to be the same as already submitted batch to trigger equivocation
		block.View = blocks[i].Block.View
		// update parentID and parentView so blocks are still connected
		block.ParentID = equivocatedBlocks[i-1].Hash()
		block.ParentView = equivocatedBlocks[i-1].View
		s.consumer.On("OnDoubleProposeDetected",
			model.BlockFromFlow(blocks[i].Block.ToHeader()), model.BlockFromFlow(block.ToHeader())).Return().Once()

		equivocatedProposals = append(equivocatedProposals, unittest.ProposalFromBlock(block))
	}
	_, err = s.cache.AddBlocks(equivocatedProposals)
	require.NoError(s.T(), err)
}

// TestBlocksAreNotConnected tests that passing a batch without sequential ordering of blocks and without gaps
// results in error.
func (s *CacheSuite) TestBlocksAreNotConnected() {
	s.Run("blocks-not-sequential", func() {
		blocks := unittest.ProposalChainFixtureFrom(10, unittest.BlockHeaderFixture())

		// shuffling blocks will break the order between them rendering batch as not sequential
		rand.Shuffle(len(blocks), func(i, j int) {
			blocks[i], blocks[j] = blocks[j], blocks[i]
		})

		_, err := s.cache.AddBlocks(blocks)
		require.ErrorIs(s.T(), err, ErrDisconnectedBatch)
	})
	s.Run("blocks-with-gaps", func() {
		blocks := unittest.ProposalChainFixtureFrom(10, unittest.BlockHeaderFixture())

		// altering Height will break ParentID in next block, rendering batch as not sequential
		blocks[len(blocks)/2].Block.Height += 1

		_, err := s.cache.AddBlocks(blocks)
		require.ErrorIs(s.T(), err, ErrDisconnectedBatch)
	})
}

// TestAddBlocksChildCertifiesParent tests a scenario: A <- B[QC_A].
// First we add A and then B, in two different batches.
// We expect that A will get certified after adding B.
func (s *CacheSuite) TestChildCertifiesParent() {
	block := unittest.BlockFixture()
	proposal := unittest.ProposalFromBlock(block)
	certifiedBatch, err := s.cache.AddBlocks([]*flow.Proposal{proposal})
	require.NoError(s.T(), err)
	require.Empty(s.T(), certifiedBatch)
	child := unittest.BlockWithParentFixture(block.ToHeader())
	certifiedBatch, err = s.cache.AddBlocks([]*flow.Proposal{unittest.ProposalFromBlock(child)})
	require.NoError(s.T(), err)
	require.Len(s.T(), certifiedBatch, 1)
	require.Equal(s.T(), block.Hash(), certifiedBatch[0].CertifyingQC.BlockID)
	require.Equal(s.T(), certifiedBatch[0].Proposal, proposal)
}

// TestChildBeforeParent tests a scenario: A <- B[QC_A].
// First we add B and then A, in two different batches.
// We expect that A will get certified after adding A.
func (s *CacheSuite) TestChildBeforeParent() {
	blocks := unittest.ProposalChainFixtureFrom(2, unittest.BlockHeaderFixture())
	_, err := s.cache.AddBlocks(blocks[1:2])
	require.NoError(s.T(), err)
	certifiedBatch, err := s.cache.AddBlocks(blocks[0:1])
	require.NoError(s.T(), err)
	require.Len(s.T(), certifiedBatch, 1)
	require.Equal(s.T(), blocks[0].Block.Hash(), certifiedBatch[0].CertifyingQC.BlockID)
	require.Equal(s.T(), blocks[0], certifiedBatch[0].Proposal)
}

// TestBlockInTheMiddle tests a scenario: A <- B[QC_A] <- C[QC_B].
// We add blocks one by one: C, A, B, we expect that after adding B, we will be able to
// certify [A, B] with QC_B as certifying QC.
func (s *CacheSuite) TestBlockInTheMiddle() {
	blocks := unittest.ProposalChainFixtureFrom(3, unittest.BlockHeaderFixture())
	// add C
	certifiedBlocks, err := s.cache.AddBlocks(blocks[2:])
	require.NoError(s.T(), err)
	require.Empty(s.T(), certifiedBlocks)

	// add A
	certifiedBlocks, err = s.cache.AddBlocks(blocks[:1])
	require.NoError(s.T(), err)
	require.Empty(s.T(), certifiedBlocks)

	// add B
	certifiedBlocks, err = s.cache.AddBlocks(blocks[1:2])
	require.NoError(s.T(), err)
	require.Len(s.T(), certifiedBlocks, 2)
	require.Equal(s.T(), blocks[0], certifiedBlocks[0].Proposal)
	require.Equal(s.T(), blocks[len(blocks)-2], certifiedBlocks[len(certifiedBlocks)-1].Proposal)
	require.Equal(s.T(), blocks[2].Block.ParentQC(), certifiedBlocks[1].CertifyingQC)
}

// TestAddBatch tests a scenario: B1 <- ... <- BN added in one batch.
// We expect that all blocks except the last one will be certified.
// Certifying QC will be taken from last block.
func (s *CacheSuite) TestAddBatch() {
	blocks := unittest.ProposalChainFixtureFrom(10, unittest.BlockHeaderFixture())
	certifiedBatch, err := s.cache.AddBlocks(blocks)
	require.NoError(s.T(), err)
	require.Len(s.T(), certifiedBatch, 9, "there should be %d - 1 certified blocks", len(blocks))
	for i := 0; i < len(certifiedBatch)-1; i++ {
		certifiedBlock := certifiedBatch[i]
		require.Equal(s.T(), blocks[i], certifiedBlock.Proposal)
		require.Equal(s.T(), blocks[i+1].Block.ParentQC(), certifiedBlock.CertifyingQC)
		require.Equal(s.T(), certifiedBlock.Proposal.Block.Hash(), certifiedBlock.CertifyingQC.BlockID)
		require.Equal(s.T(), certifiedBlock.Proposal.Block.View, certifiedBlock.CertifyingQC.View)
	}
}

// TestDuplicatedBatch checks that processing redundant inputs rejects batches where all blocks
// already reside in the cache. Batches that have at least one new block should be accepted.
func (s *CacheSuite) TestDuplicatedBatch() {
	blocks := unittest.ProposalChainFixtureFrom(10, unittest.BlockHeaderFixture())

	certifiedBatch, err := s.cache.AddBlocks(blocks[1:])
	require.NoError(s.T(), err)
	require.Len(s.T(), certifiedBatch, len(blocks)-2)
	require.Equal(s.T(), blocks[1], certifiedBatch[0].Proposal)
	require.Equal(s.T(), blocks[len(blocks)-2], certifiedBatch[len(certifiedBatch)-1].Proposal)
	require.Equal(s.T(), blocks[len(blocks)-1].Block.ParentQC(), certifiedBatch[len(certifiedBatch)-1].CertifyingQC)

	// add same batch again, this has to be rejected as redundant input
	certifiedBatch, err = s.cache.AddBlocks(blocks[1:])
	require.NoError(s.T(), err)
	require.Empty(s.T(), certifiedBatch)

	// add batch with one extra leading block, this should be accepted even though 9 out of 10 blocks
	// were already processed
	certifiedBatch, err = s.cache.AddBlocks(blocks)
	require.NoError(s.T(), err)
	require.Len(s.T(), certifiedBatch, len(blocks)-1)
	require.Equal(s.T(), blocks[0], certifiedBatch[0].Proposal)
	require.Equal(s.T(), blocks[len(blocks)-2], certifiedBatch[len(certifiedBatch)-1].Proposal)
	require.Equal(s.T(), blocks[len(blocks)-1].Block.ParentQC(), certifiedBatch[len(certifiedBatch)-1].CertifyingQC)
}

// TestPruneUpToView tests that blocks lower than pruned height will be properly filtered out from incoming batch.
func (s *CacheSuite) TestPruneUpToView() {
	blocks := unittest.ProposalChainFixtureFrom(3, unittest.BlockHeaderFixture())
	s.cache.PruneUpToView(blocks[1].Block.View)
	certifiedBatch, err := s.cache.AddBlocks(blocks)
	require.NoError(s.T(), err)
	require.Equal(s.T(), blocks[1], certifiedBatch[0].Proposal)
	require.Equal(s.T(), blocks[len(blocks)-2], certifiedBatch[len(certifiedBatch)-1].Proposal)
	require.Equal(s.T(), blocks[len(blocks)-1].Block.ParentQC(), certifiedBatch[len(certifiedBatch)-1].CertifyingQC)
}

// TestConcurrentAdd simulates multiple workers adding batches of blocks out of order.
// We use the following setup:
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
	blocks := unittest.ProposalChainFixtureFrom(workers*blocksPerWorker, unittest.BlockHeaderFixture())

	var wg sync.WaitGroup
	wg.Add(workers)

	var certifiedBlocksLock sync.Mutex
	var allCertifiedBlocks []flow.CertifiedBlock
	for i := 0; i < workers; i++ {
		go func(blocks []*flow.Proposal) {
			defer wg.Done()
			for batch := 0; batch < batchesPerWorker; batch++ {
				certifiedBlocks, err := s.cache.AddBlocks(blocks[batch*blocksPerBatch : (batch+1)*blocksPerBatch])
				require.NoError(s.T(), err)
				certifiedBlocksLock.Lock()
				allCertifiedBlocks = append(allCertifiedBlocks, certifiedBlocks...)
				certifiedBlocksLock.Unlock()
			}
		}(blocks[i*blocksPerWorker : (i+1)*blocksPerWorker])
	}

	unittest.RequireReturnsBefore(s.T(), wg.Wait, time.Millisecond*500, "should submit blocks before timeout")

	require.Len(s.T(), allCertifiedBlocks, len(blocks)-1)
	slices.SortFunc(allCertifiedBlocks, func(lhs flow.CertifiedBlock, rhs flow.CertifiedBlock) int {
		return int(lhs.Proposal.Block.Height) - int(rhs.Proposal.Block.Height)
	})
	for i, block := range blocks[:len(blocks)-1] {
		require.Equal(s.T(), block, allCertifiedBlocks[i].Proposal)
	}
}

// TestSecondaryIndexCleanup tests if ejected entities are correctly cleaned up from secondary index
func (s *CacheSuite) TestSecondaryIndexCleanup() {
	// create blocks more than limit
	blocks := unittest.ProposalChainFixtureFrom(2*defaultHeroCacheLimit, unittest.BlockHeaderFixture())
	_, err := s.cache.AddBlocks(blocks)
	require.NoError(s.T(), err)
	require.Len(s.T(), s.cache.byView, defaultHeroCacheLimit)
	require.Len(s.T(), s.cache.byParent, defaultHeroCacheLimit)
}

// TestMultipleChildrenForSameParent tests a scenario where we have:
// /  A <- B
// /    <- C
// We insert:
// 1. [B]
// 2. [C]
// 3. [A]
// We should be able to certify A since B and C are in cache, any QC will work.
func (s *CacheSuite) TestMultipleChildrenForSameParent() {
	A := unittest.BlockFixture()
	B := unittest.BlockWithParentFixture(A.ToHeader())
	C := unittest.BlockWithParentFixture(A.ToHeader())
	C.View = B.View + 1 // make sure views are different
	Ap := unittest.ProposalFromBlock(A)
	Bp := unittest.ProposalFromBlock(B)
	Cp := unittest.ProposalFromBlock(C)

	_, err := s.cache.AddBlocks([]*flow.Proposal{Bp})
	require.NoError(s.T(), err)
	_, err = s.cache.AddBlocks([]*flow.Proposal{Cp})
	require.NoError(s.T(), err)
	certifiedBlocks, err := s.cache.AddBlocks([]*flow.Proposal{Ap})
	require.NoError(s.T(), err)
	require.Len(s.T(), certifiedBlocks, 1)
	require.Equal(s.T(), Ap, certifiedBlocks[0].Proposal)
	require.Equal(s.T(), A.Hash(), certifiedBlocks[0].CertifyingQC.BlockID)
}

// TestChildEjectedBeforeAddingParent tests a scenario where we have:
// /  A <- B
// /    <- C
// We insert:
// 1. [B]
// 2. [C]
// 3. [A]
// Between 2. and 3. B gets ejected, we should be able to certify A since C is still in cache.
func (s *CacheSuite) TestChildEjectedBeforeAddingParent() {
	A := unittest.BlockFixture()
	B := unittest.BlockWithParentFixture(A.ToHeader())
	C := unittest.BlockWithParentFixture(A.ToHeader())
	C.View = B.View + 1 // make sure views are different

	Ap := unittest.ProposalFromBlock(A)
	Bp := unittest.ProposalFromBlock(B)
	Cp := unittest.ProposalFromBlock(C)
	_, err := s.cache.AddBlocks([]*flow.Proposal{Bp})
	require.NoError(s.T(), err)
	_, err = s.cache.AddBlocks([]*flow.Proposal{Cp})
	require.NoError(s.T(), err)
	// eject B
	s.cache.backend.Remove(B.Hash())
	s.cache.handleEjectedBlock(Bp)

	certifiedBlocks, err := s.cache.AddBlocks([]*flow.Proposal{Ap})
	require.NoError(s.T(), err)
	require.Len(s.T(), certifiedBlocks, 1)
	require.Equal(s.T(), Ap, certifiedBlocks[0].Proposal)
	require.Equal(s.T(), A.Hash(), certifiedBlocks[0].CertifyingQC.BlockID)
}

// TestAddOverCacheLimit tests a scenario where caller feeds blocks to the cache in concurrent way
// largely exceeding internal cache capacity leading to ejection of large number of blocks.
// Expect to eventually certify all possible blocks assuming producer continue to push same blocks over and over again.
// This test scenario emulates sync engine pushing blocks from other committee members.
func (s *CacheSuite) TestAddOverCacheLimit() {
	// create blocks more than limit
	workers := 10
	blocksPerWorker := 10
	s.cache = NewCache(unittest.Logger(), uint32(blocksPerWorker), metrics.NewNoopCollector(), s.consumer)

	blocks := unittest.ProposalChainFixtureFrom(blocksPerWorker*workers, unittest.BlockHeaderFixture())

	var uniqueBlocksLock sync.Mutex
	// AddBlocks can certify same blocks, especially when we push same blocks over and over
	// use a map to track those. Using a lock to provide concurrency safety.
	uniqueBlocks := make(map[flow.Identifier]struct{}, 0)

	// all workers will submit blocks unless condition is satisfied
	// whenever len(uniqueBlocks) == certifiedGoal it means we have certified all available blocks.
	done := atomic.NewBool(false)
	certifiedGoal := len(blocks) - 1

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(blocks []*flow.Proposal) {
			defer wg.Done()
			for !done.Load() {
				// worker submits blocks while condition is not satisfied
				for _, block := range blocks {
					// push blocks one by one, pairing with randomness of scheduler
					// blocks will be delivered chaotically
					certifiedBlocks, err := s.cache.AddBlocks([]*flow.Proposal{block})
					require.NoError(s.T(), err)
					if len(certifiedBlocks) > 0 {
						uniqueBlocksLock.Lock()
						for _, block := range certifiedBlocks {
							uniqueBlocks[block.BlockID()] = struct{}{}
						}
						if len(uniqueBlocks) == certifiedGoal {
							done.Store(true)
						}
						uniqueBlocksLock.Unlock()
					}
				}
			}
		}(blocks[i*blocksPerWorker : (i+1)*blocksPerWorker])
	}
	unittest.RequireReturnsBefore(s.T(), wg.Wait, time.Millisecond*500, "should submit blocks before timeout")
}

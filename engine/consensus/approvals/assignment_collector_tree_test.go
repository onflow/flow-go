package approvals_test

import (
	"fmt"
	"sync"
	"testing"

	mock2 "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/consensus/approvals"
	"github.com/onflow/flow-go/engine/consensus/approvals/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// AssignmentCollectorTreeSuite performs isolated testing of AssignmentCollectorTree
func TestAssignmentCollectorTree(t *testing.T) {
	suite.Run(t, new(AssignmentCollectorTreeSuite))
}

type mockedCollectorWrapper struct {
	collector *mock.AssignmentCollector
	status    approvals.ProcessingStatus
}

type AssignmentCollectorTreeSuite struct {
	approvals.BaseAssignmentCollectorTestSuite

	collectorTree    *approvals.AssignmentCollectorTree
	factoryMethod    approvals.NewCollectorFactoryMethod
	mockedCollectors map[flow.Identifier]*mockedCollectorWrapper
}

func (s *AssignmentCollectorTreeSuite) SetupTest() {
	s.BaseAssignmentCollectorTestSuite.SetupTest()

	s.factoryMethod = func(result *flow.ExecutionResult) (approvals.AssignmentCollector, error) {
		if wrapper, found := s.mockedCollectors[result.ID()]; found {
			return wrapper.collector, nil
		}
		return nil, fmt.Errorf("mocked collector %v not found", result.ID())
	}

	s.mockedCollectors = make(map[flow.Identifier]*mockedCollectorWrapper)
	s.collectorTree = approvals.NewAssignmentCollectorTree(&s.ParentBlock, s.Headers, s.factoryMethod)

	s.prepareMockedCollector(s.IncorporatedResult.Result)
}

// prepareMockedCollector prepares a mocked collector and stores it in map, later it will be used
// to create new collector when factory method will be called
func (s *AssignmentCollectorTreeSuite) prepareMockedCollector(result *flow.ExecutionResult) *mockedCollectorWrapper {
	collector := &mock.AssignmentCollector{}
	collector.On("ResultID").Return(result.ID()).Maybe()
	collector.On("Result").Return(result).Maybe()
	collector.On("BlockID").Return(result.BlockID).Maybe()
	collector.On("Block").Return(func() *flow.Header {
		return s.Blocks[result.BlockID]
	}).Maybe()

	wrapper := &mockedCollectorWrapper{
		collector: collector,
		status:    approvals.CachingApprovals,
	}

	collector.On("ProcessingStatus").Return(func() approvals.ProcessingStatus {
		return wrapper.status
	})
	s.mockedCollectors[result.ID()] = wrapper
	return wrapper
}

func mockCollectorStateTransition(wrapper *mockedCollectorWrapper, oldState, newState approvals.ProcessingStatus) {
	wrapper.collector.On("ChangeProcessingStatus",
		oldState, newState).Return(nil).Run(func(args mock2.Arguments) {
		wrapper.status = newState
	}).Once()
}

// TestGetSize_ConcurrentAccess tests if assignment collector tree correctly returns size when concurrently adding
// items
func (s *AssignmentCollectorTreeSuite) TestGetSize_ConcurrentAccess() {
	numberOfWorkers := 10
	batchSize := 10
	chain := unittest.ChainFixtureFrom(numberOfWorkers*batchSize, &s.IncorporatedBlock)
	result0 := unittest.ExecutionResultFixture()
	receipts := unittest.ReceiptChainFor(chain, result0)
	for _, block := range chain {
		s.Blocks[block.ID()] = block.Header
	}
	for _, receipt := range receipts {
		s.prepareMockedCollector(&receipt.ExecutionResult)
	}

	var wg sync.WaitGroup
	wg.Add(numberOfWorkers)
	for worker := 0; worker < numberOfWorkers; worker++ {
		go func(workerIndex int) {
			defer wg.Done()
			for i := 0; i < batchSize; i++ {
				result := &receipts[workerIndex*batchSize+i].ExecutionResult
				collector, err := s.collectorTree.GetOrCreateCollector(result)
				require.NoError(s.T(), err)
				require.True(s.T(), collector.Created)
			}
		}(worker)
	}
	wg.Wait()

	require.Equal(s.T(), uint64(len(receipts)), s.collectorTree.GetSize())
}

// TestGetCollector tests basic case where previously created collector can be retrieved
func (s *AssignmentCollectorTreeSuite) TestGetCollector() {
	result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
		result.BlockID = s.IncorporatedBlock.ID()
	})
	s.prepareMockedCollector(result)
	expectedCollector, err := s.collectorTree.GetOrCreateCollector(result)
	require.NoError(s.T(), err)
	require.True(s.T(), expectedCollector.Created)
	collector := s.collectorTree.GetCollector(result.ID())
	require.Equal(s.T(), collector, expectedCollector.Collector)

	// get collector for unknown result ID should return nil
	collector = s.collectorTree.GetCollector(unittest.IdentifierFixture())
	require.Nil(s.T(), collector)
}

// TestGetCollectorsByInterval tests that GetCollectorsByInterval returns expected array of AssignmentCollector
// in proposed interval
func (s *AssignmentCollectorTreeSuite) TestGetCollectorsByInterval() {
	chain := unittest.ChainFixtureFrom(10, &s.ParentBlock)
	receipts := unittest.ReceiptChainFor(chain, s.IncorporatedResult.Result)
	for _, block := range chain {
		s.Blocks[block.ID()] = block.Header
	}

	// process all receipts except first one, this will build a chain of collectors but all of them will be
	// in caching state
	for index, receipt := range receipts {
		result := &receipt.ExecutionResult
		mockedCollector := s.prepareMockedCollector(result)
		mockCollectorStateTransition(mockedCollector, approvals.CachingApprovals, approvals.VerifyingApprovals)
		if index > 0 {
			createdCollector, err := s.collectorTree.GetOrCreateCollector(result)
			require.NoError(s.T(), err)
			require.True(s.T(), createdCollector.Created)
		}
	}

	collectors := s.collectorTree.GetCollectorsByInterval(0, s.Block.Height+100)
	require.Empty(s.T(), collectors)

	_, err := s.collectorTree.GetOrCreateCollector(&receipts[0].ExecutionResult)
	require.NoError(s.T(), err)

	collectors = s.collectorTree.GetCollectorsByInterval(0, s.Block.Height+100)
	require.Len(s.T(), collectors, len(receipts))
}

// TestGetOrCreateCollector tests that getting collector creates one on first call and returns from cache on second one.
func (s *AssignmentCollectorTreeSuite) TestGetOrCreateCollector_ReturnFromCache() {
	result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
		result.BlockID = s.IncorporatedBlock.ID()
	})
	s.prepareMockedCollector(result)
	lazyCollector, err := s.collectorTree.GetOrCreateCollector(result)
	require.NoError(s.T(), err)
	require.True(s.T(), lazyCollector.Created)
	require.Equal(s.T(), approvals.CachingApprovals, lazyCollector.Collector.ProcessingStatus())

	lazyCollector, err = s.collectorTree.GetOrCreateCollector(result)
	require.NoError(s.T(), err)
	// should be returned from cache
	require.False(s.T(), lazyCollector.Created)
}

// TestGetOrCreateCollector_FactoryError tests that AssignmentCollectorTree correctly handles factory method error
func (s *AssignmentCollectorTreeSuite) TestGetOrCreateCollector_FactoryError() {
	result := unittest.ExecutionResultFixture()
	lazyCollector, err := s.collectorTree.GetOrCreateCollector(result)
	require.Error(s.T(), err)
	require.Nil(s.T(), lazyCollector)
}

// TestGetOrCreateCollector_CollectorParentIsSealed tests a case where we add a result for collector with sealed parent.
// In this specific case collector has to become verifying instead of caching.
func (s *AssignmentCollectorTreeSuite) TestGetOrCreateCollector_CollectorParentIsSealed() {
	result := s.IncorporatedResult.Result
	mockCollectorStateTransition(s.mockedCollectors[result.ID()],
		approvals.CachingApprovals, approvals.VerifyingApprovals)
	lazyCollector, err := s.collectorTree.GetOrCreateCollector(result)
	require.NoError(s.T(), err)
	require.True(s.T(), lazyCollector.Created)
	require.Equal(s.T(), approvals.VerifyingApprovals, lazyCollector.Collector.ProcessingStatus())
}

// TestGetOrCreateCollector_AddingFinalizedCollector tests a case when we are trying to add collector which is already finalized.
// Leveled forest doesn't accept vertexes lower than the lowest height.
func (s *AssignmentCollectorTreeSuite) TestGetOrCreateCollector_AddingFinalizedCollector() {
	block := unittest.BlockFixture()
	block.Header.Height = s.ParentBlock.Height - 10
	result := unittest.ExecutionResultFixture(unittest.WithBlock(&block))
	lazyCollector, err := s.collectorTree.GetOrCreateCollector(result)
	require.Error(s.T(), err)
	require.Nil(s.T(), lazyCollector)
}

// TestFinalizeForkAtLevel_ProcessableAfterSealedParent tests scenario that finalized collector becomes processable
// after parent block gets sealed. More specifically this case:
// P <- A <- B[ER{A}] <- C[ER{B}] <- D[ER{C}]
//        <- E[ER{A}] <- F[ER{E}] <- G[ER{F}]
//               |
//           finalized
// Initially P was executed,  B is finalized and incorporates ER for A, C incorporates ER for B, D was forked from A,
// but wasn't finalized, E incorporates ER for D.
// Let's take a case where we have collectors for ER incorporated in blocks B, C, D, E. Since we don't
// have a collector for A, {B, C, D, E} are not processable. Test that when A becomes sealed {B, C, D} become processable
// but E is unprocessable since D wasn't part of finalized fork.
func (s *AssignmentCollectorTreeSuite) TestFinalizeForkAtLevel_ProcessableAfterSealedParent() {
	s.IdentitiesCache[s.IncorporatedBlock.ID()] = s.AuthorizedVerifiers
	// two forks
	forks := make([][]*flow.Block, 2)
	results := make([][]*flow.IncorporatedResult, 2)
	for i := 0; i < len(forks); i++ {
		fork := unittest.ChainFixtureFrom(3, &s.IncorporatedBlock)
		forks[i] = fork
		prevResult := s.IncorporatedResult.Result
		// create execution results for all blocks except last one, since it won't be valid by definition
		for _, block := range fork {
			blockID := block.ID()

			// create execution result for previous block in chain
			// this result will be incorporated in current block.
			result := unittest.ExecutionResultFixture(
				unittest.WithPreviousResult(*prevResult),
			)
			result.BlockID = block.Header.ParentID

			// update caches
			s.Blocks[blockID] = block.Header
			s.IdentitiesCache[blockID] = s.AuthorizedVerifiers

			IR := unittest.IncorporatedResult.Fixture(
				unittest.IncorporatedResult.WithResult(result),
				unittest.IncorporatedResult.WithIncorporatedBlockID(blockID))

			results[i] = append(results[i], IR)

			s.prepareMockedCollector(result)

			collector, err := s.collectorTree.GetOrCreateCollector(IR.Result)
			require.NoError(s.T(), err)

			require.Equal(s.T(), approvals.CachingApprovals, collector.Collector.ProcessingStatus())

			prevResult = result
		}
	}

	finalized := forks[0][0].Header

	s.MarkFinalized(&s.IncorporatedBlock)
	s.MarkFinalized(finalized)

	// at this point collectors for forks[0] should be processable and for forks[1] not
	for forkIndex := range forks {
		for _, result := range results[forkIndex] {
			wrapper, found := s.mockedCollectors[result.Result.ID()]
			require.True(s.T(), found)

			if forkIndex == 0 {
				mockCollectorStateTransition(wrapper, approvals.CachingApprovals, approvals.VerifyingApprovals)
			} else {
				mockCollectorStateTransition(wrapper, approvals.CachingApprovals, approvals.Orphaned)
			}
		}
	}

	// A becomes sealed, B becomes finalized
	err := s.collectorTree.FinalizeForkAtLevel(finalized, &s.Block)
	require.NoError(s.T(), err)
}

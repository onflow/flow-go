package approvals_test

import (
	"sync"
	"testing"

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

type AssignmentCollectorTreeSuite struct {
	approvals.BaseAssignmentCollectorTestSuite

	collectorTree    *approvals.AssignmentCollectorTree
	factoryMethod    approvals.NewCollectorFactoryMethod
	mockedCollectors map[flow.Identifier]*mock.AssignmentCollector
}

func (s *AssignmentCollectorTreeSuite) SetupTest() {
	s.BaseAssignmentCollectorTestSuite.SetupTest()

	s.factoryMethod = func(result *flow.ExecutionResult) (approvals.AssignmentCollector, error) {
		return s.mockedCollectors[result.ID()], nil
	}

	s.mockedCollectors = make(map[flow.Identifier]*mock.AssignmentCollector)
	s.collectorTree = approvals.NewAssignmentCollectorTree(&s.ParentBlock, s.Headers, s.factoryMethod)

	s.prepareMockedCollector(s.IncorporatedResult.Result)
}

// prepareMockedCollector prepares a mocked collector and stores it in map, later it will be used
// to create new collector when factory method will be called
func (s *AssignmentCollectorTreeSuite) prepareMockedCollector(result *flow.ExecutionResult) *mock.AssignmentCollector {
	collector := &mock.AssignmentCollector{}
	collector.On("ResultID").Return(result.ID()).Maybe()
	collector.On("Result").Return(result).Maybe()
	collector.On("BlockID").Return(result.BlockID).Maybe()
	collector.On("Block").Return(func() *flow.Header {
		return s.Blocks[result.BlockID]
	}).Maybe()
	collector.On("ProcessingStatus").Return(approvals.CachingApprovals)
	s.mockedCollectors[result.ID()] = collector
	return collector
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
			collector, found := s.mockedCollectors[result.Result.ID()]
			require.True(s.T(), found)
			if forkIndex == 0 {
				collector.On("ChangeProcessingStatus", approvals.CachingApprovals, approvals.VerifyingApprovals).Return(nil).Once()
			} else {
				collector.On("ChangeProcessingStatus", approvals.CachingApprovals, approvals.Orphaned).Return(nil).Once()
			}
		}
	}

	// A becomes sealed, B becomes finalized
	err := s.collectorTree.FinalizeForkAtLevel(finalized, &s.Block)
	require.NoError(s.T(), err)
}

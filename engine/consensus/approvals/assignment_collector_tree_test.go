package approvals_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	mocktestify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus/approvals"
	mockAC "github.com/onflow/flow-go/engine/consensus/approvals/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// AssignmentCollectorTreeSuite performs isolated testing of AssignmentCollectorTree.
// AssignmentCollectorTreeSuite has to correctly create collectors, cache them and maintain pruned state
// based on finalization and sealing events.
func TestAssignmentCollectorTree(t *testing.T) {
	suite.Run(t, new(AssignmentCollectorTreeSuite))
}

var factoryError = errors.New("factory error")

// mockedCollectorWrapper is a helper structure for holding mock.AssignmentCollector and ProcessingStatus
// this is needed for simplifying mocking of state transitions.
type mockedCollectorWrapper struct {
	collector *mockAC.AssignmentCollector
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
		return nil, fmt.Errorf("mocked collector %v not found: %w", result.ID(), factoryError)
	}

	s.mockedCollectors = make(map[flow.Identifier]*mockedCollectorWrapper)
	s.collectorTree = approvals.NewAssignmentCollectorTree(&s.ParentBlock, s.Headers, s.factoryMethod)

	s.prepareMockedCollector(s.IncorporatedResult.Result)
}

// prepareMockedCollector prepares a mocked collector and stores it in map, later it will be used
// to create new collector when factory method will be called
func (s *AssignmentCollectorTreeSuite) prepareMockedCollector(result *flow.ExecutionResult) *mockedCollectorWrapper {
	collector := &mockAC.AssignmentCollector{}
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

// requireStateTransition specifies that we are expecting the business logic to
// execute the specified state transition.
func requireStateTransition(wrapper *mockedCollectorWrapper, oldState, newState approvals.ProcessingStatus) {
	fmt.Printf("Require state transition for %x %v -> %v\n", wrapper.collector.BlockID(), wrapper.status, newState)
	wrapper.collector.On("ChangeProcessingStatus",
		oldState, newState).Return(nil).Run(func(args mocktestify.Arguments) {
		fmt.Printf("Performing state transition for %x %v -> %v\n", wrapper.collector.BlockID(), wrapper.status, newState)
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

// TestGetCollectorsByInterval tests that GetCollectorsByInterval returns a slice
// with the AssignmentCollectors from the requested interval
func (s *AssignmentCollectorTreeSuite) TestGetCollectorsByInterval() {
	chain := unittest.ChainFixtureFrom(10, &s.ParentBlock)
	receipts := unittest.ReceiptChainFor(chain, s.IncorporatedResult.Result)
	for _, block := range chain {
		s.Blocks[block.ID()] = block.Header
	}

	// Process all receipts except first one. This generates a chain of collectors but all of them will be
	// in caching state, as the receipt connecting them to the sealed state has not been added yet.
	// As `GetCollectorsByInterval` only returns verifying collectors, we expect an empty slice to be returned.
	for index, receipt := range receipts {
		result := &receipt.ExecutionResult
		s.prepareMockedCollector(result)
		if index > 0 {
			createdCollector, err := s.collectorTree.GetOrCreateCollector(result)
			require.NoError(s.T(), err)
			require.True(s.T(), createdCollector.Created)
		}
	}
	collectors := s.collectorTree.GetCollectorsByInterval(0, s.Block.Height+100)
	require.Empty(s.T(), collectors)

	// Now we add the connecting receipt. The AssignmentCollectorTree should then change the states
	// of all added collectors from `CachingApprovals` to `VerifyingApprovals`. Therefore, we
	// expect `GetCollectorsByInterval` to return all added collectors.
	for _, receipt := range receipts {
		mockedCollector := s.mockedCollectors[receipt.ExecutionResult.ID()]
		requireStateTransition(mockedCollector, approvals.CachingApprovals, approvals.VerifyingApprovals)
	}
	_, err := s.collectorTree.GetOrCreateCollector(&receipts[0].ExecutionResult)
	require.NoError(s.T(), err)

	collectors = s.collectorTree.GetCollectorsByInterval(0, s.Block.Height+100)
	require.Len(s.T(), collectors, len(receipts))

	for _, receipt := range receipts {
		mockedCollector := s.mockedCollectors[receipt.ExecutionResult.ID()]
		mockedCollector.collector.AssertExpectations(s.T())
	}
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

	lazyCollector2, err := s.collectorTree.GetOrCreateCollector(result)
	require.NoError(s.T(), err)
	// should be returned from cache
	require.False(s.T(), lazyCollector2.Created)
	require.True(s.T(), lazyCollector.Collector == lazyCollector2.Collector)
}

// TestGetOrCreateCollector_FactoryError tests that AssignmentCollectorTree correctly handles factory method error
func (s *AssignmentCollectorTreeSuite) TestGetOrCreateCollector_FactoryError() {
	result := unittest.ExecutionResultFixture()
	lazyCollector, err := s.collectorTree.GetOrCreateCollector(result)
	require.ErrorIs(s.T(), err, factoryError)
	require.Nil(s.T(), lazyCollector)
}

// TestGetOrCreateCollector_CollectorParentIsSealed tests a case where we add a result for collector with sealed parent.
// In this specific case collector has to become verifying instead of caching.
func (s *AssignmentCollectorTreeSuite) TestGetOrCreateCollector_CollectorParentIsSealed() {
	result := s.IncorporatedResult.Result
	requireStateTransition(s.mockedCollectors[result.ID()],
		approvals.CachingApprovals, approvals.VerifyingApprovals)
	lazyCollector, err := s.collectorTree.GetOrCreateCollector(result)
	require.NoError(s.T(), err)
	require.True(s.T(), lazyCollector.Created)
	require.Equal(s.T(), approvals.VerifyingApprovals, lazyCollector.Collector.ProcessingStatus())
}

// TestGetOrCreateCollector_AddingSealedCollector tests a case when we are trying to add collector which is already sealed.
// Leveled forest doesn't accept vertexes lower than the lowest height.
func (s *AssignmentCollectorTreeSuite) TestGetOrCreateCollector_AddingSealedCollector() {
	block := unittest.BlockWithParentFixture(&s.ParentBlock)
	s.Blocks[block.ID()] = block.Header
	result := unittest.ExecutionResultFixture(unittest.WithBlock(&block))
	s.prepareMockedCollector(result)

	// generate a few sealed blocks
	prevSealedBlock := block.Header
	for i := 0; i < 5; i++ {
		sealedBlock := unittest.BlockHeaderWithParentFixture(prevSealedBlock)
		s.MarkFinalized(&sealedBlock)
		_ = s.collectorTree.FinalizeForkAtLevel(&sealedBlock, &sealedBlock)
	}

	// now adding a collector which is lower than sealed height should result in error
	lazyCollector, err := s.collectorTree.GetOrCreateCollector(result)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsOutdatedInputError(err))
	require.Nil(s.T(), lazyCollector)
}

// TestFinalizeForkAtLevel_ProcessableAfterSealedParent tests scenario that finalized collector becomes processable
// after parent block gets sealed. More specifically this case:
// P <- A <- B[ER{A}] <- C[ER{B}] <- D[ER{C}]
//        <- E[ER{A}] <- F[ER{E}] <- G[ER{F}]
//               |
//           finalized
// Initially P was executed, B is finalized and incorporates ER for A, C incorporates ER for B, D incorporates ER for C,
// E was forked from A and incorporates ER for A, but wasn't finalized, F incorporates ER for E, G incorporates ER for F
// Let's take a case where we have collectors for execution results ER{A}, ER{B}, ER{C}, ER{E}, ER{F}.
// All of those collectors are not processable because ER{A} doesn't have parent collector or is not a parent of sealed block.
// Test that when A becomes sealed {ER{B}, ER{C}} become processable
// but {ER{E}, ER{F}} are unprocessable since E wasn't part of finalized fork.
func (s *AssignmentCollectorTreeSuite) TestFinalizeForkAtLevel_ProcessableAfterSealedParent() {
	s.IdentitiesCache[s.IncorporatedBlock.ID()] = s.AuthorizedVerifiers
	// two forks
	forks := make([][]*flow.Block, 2)
	results := make([][]*flow.IncorporatedResult, 2)

	firstResult := unittest.ExecutionResultFixture(
		unittest.WithPreviousResult(*s.IncorporatedResult.Result),
		unittest.WithExecutionResultBlockID(s.IncorporatedBlock.ID()))
	s.prepareMockedCollector(firstResult)
	for i := 0; i < len(forks); i++ {
		fork := unittest.ChainFixtureFrom(3, &s.IncorporatedBlock)
		forks[i] = fork
		prevResult := firstResult
		// create execution results for all blocks except last one, since it won't be valid by definition
		for _, block := range fork {
			blockID := block.ID()

			// update caches
			s.Blocks[blockID] = block.Header
			s.IdentitiesCache[blockID] = s.AuthorizedVerifiers

			IR := unittest.IncorporatedResult.Fixture(
				unittest.IncorporatedResult.WithResult(prevResult),
				unittest.IncorporatedResult.WithIncorporatedBlockID(blockID))

			results[i] = append(results[i], IR)

			collector, err := s.collectorTree.GetOrCreateCollector(IR.Result)
			require.NoError(s.T(), err)

			require.Equal(s.T(), approvals.CachingApprovals, collector.Collector.ProcessingStatus())

			// create execution result for previous block in chain
			// this result will be incorporated in current block.
			prevResult = unittest.ExecutionResultFixture(
				unittest.WithPreviousResult(*prevResult),
				unittest.WithExecutionResultBlockID(blockID),
			)
			s.prepareMockedCollector(prevResult)
		}
	}

	finalized := forks[0][0].Header

	s.MarkFinalized(&s.IncorporatedBlock)
	s.MarkFinalized(finalized)

	// at this point collectors for forks[0] should be processable and for forks[1] not
	for forkIndex := range forks {
		for resultIndex, result := range results[forkIndex] {
			wrapper, found := s.mockedCollectors[result.Result.ID()]
			fmt.Printf("forkIndex: %d, resultIndex: %d, id: %x, result: %v\n", forkIndex, resultIndex, result.Result.ID(), result.Result)
			require.True(s.T(), found)

			if forkIndex == 0 {
				requireStateTransition(wrapper, approvals.CachingApprovals, approvals.VerifyingApprovals)
			} else if resultIndex > 0 {
				// first result shouldn't transfer to orphaned state since it's actually ER{A} which is part of finalized fork.
				requireStateTransition(wrapper, approvals.CachingApprovals, approvals.Orphaned)
			}
		}
	}

	// A becomes sealed, B becomes finalized
	err := s.collectorTree.FinalizeForkAtLevel(finalized, &s.Block)
	require.NoError(s.T(), err)

	for forkIndex := range forks {
		for _, result := range results[forkIndex] {
			s.mockedCollectors[result.Result.ID()].collector.AssertExpectations(s.T())
		}
	}
}

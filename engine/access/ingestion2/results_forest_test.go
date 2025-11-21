package ingestion2_test

import (
	"sync"
	"testing"
	"testing/synctest"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/access/ingestion2"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	opsyncmock "github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/mock"
	forestmodule "github.com/onflow/flow-go/module/forest"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

// ResultsForestSuite tests the ResultsForest component following best practices
// from the guidelines. It provides comprehensive coverage for all exported methods
// and tests error conditions, concurrency, and invariant violations.
type ResultsForestSuite struct {
	suite.Suite

	g *fixtures.GeneratorSuite

	log                         zerolog.Logger
	headers                     *storagemock.Headers
	executionResults            *storagemock.ExecutionResults
	pipelineFactory             *opsyncmock.PipelineFactory
	latestPersistedSealedResult *storagemock.LatestPersistedSealedResultReader
	maxViewDelta                uint64

	// initial sealed result and header are what's stored in the latest persisted sealed result.
	// they are not included in `blocks` or `results`.
	initialSealedHeader *flow.Header
	initialSealedResult *flow.ExecutionResult

	// blocks and results are the unprocessed blocks and results that are part of the main chain.
	blocks  []*flow.Block
	results []*flow.ExecutionResult

	// allBlocks holds all blocks available within storage. this includes `s.blocks` as well as any
	// blocks generated during the tests that are not part of the main chain.
	allBlocks map[flow.Identifier]*flow.Block
}

func TestResultsForest(t *testing.T) {
	suite.Run(t, new(ResultsForestSuite))
}

func (s *ResultsForestSuite) SetupTest() {
	s.log = unittest.Logger()
	s.headers = storagemock.NewHeaders(s.T())
	s.executionResults = storagemock.NewExecutionResults(s.T())
	s.pipelineFactory = opsyncmock.NewPipelineFactory(s.T())
	s.latestPersistedSealedResult = storagemock.NewLatestPersistedSealedResultReader(s.T())
	s.maxViewDelta = 1000

	s.g = fixtures.NewGeneratorSuite()
	resultGen := s.g.ExecutionResults()

	initialSealedBlock := s.g.Blocks().Fixture()
	s.initialSealedHeader = initialSealedBlock.ToHeader()
	s.initialSealedResult = resultGen.Fixture(resultGen.WithBlock(initialSealedBlock))

	blockCount := 4
	s.allBlocks = make(map[flow.Identifier]*flow.Block, blockCount)
	s.blocks, s.results = s.chainWithSeals(blockCount, s.initialSealedHeader, s.initialSealedResult.ID())

	// Setup blackbox mock for the headers
	s.headers.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(
		func(blockID flow.Identifier) (*flow.Header, error) {
			for _, block := range s.allBlocks {
				if block.ID() == blockID {
					return block.ToHeader(), nil
				}
			}
			return nil, storage.ErrNotFound
		},
	).Maybe()
}

// TestNewResultsForest tests the constructor of ResultsForest.
// It verifies successful creation with valid parameters and error handling
// when header retrieval fails.
func (s *ResultsForestSuite) TestNewResultsForest() {
	s.latestPersistedSealedResult.
		On("Latest").
		Return(s.initialSealedResult.ID(), s.initialSealedHeader.Height)

	s.Run("successful creation", func() {
		s.executionResults.
			On("ByID", s.initialSealedResult.ID()).
			Return(s.initialSealedResult, nil).
			Once()

		s.headers.
			On("ByHeight", s.initialSealedHeader.Height).
			Return(s.initialSealedHeader, nil).
			Once()

		pipeline := opsyncmock.NewPipeline(s.T())
		pipeline.On("SetSealed").Return().Once()
		s.pipelineFactory.
			On("NewCompletedPipeline", s.initialSealedResult).
			Return(pipeline).
			Once()

		forest, err := ingestion2.NewResultsForest(
			s.log,
			s.headers,
			s.executionResults,
			s.pipelineFactory,
			s.latestPersistedSealedResult,
			s.maxViewDelta,
		)
		s.Require().NoError(err)

		s.NotNil(forest)
		s.Equal(s.initialSealedHeader.View, forest.LowestView())
	})

	s.Run("result retrieval fails", func() {
		s.executionResults.
			On("ByID", s.initialSealedResult.ID()).
			Return(nil, storage.ErrNotFound).
			Once()

		forest, err := ingestion2.NewResultsForest(
			s.log,
			s.headers,
			s.executionResults,
			s.pipelineFactory,
			s.latestPersistedSealedResult,
			s.maxViewDelta,
		)
		s.Error(err)
		s.Nil(forest)
	})

	s.Run("header retrieval fails", func() {
		s.executionResults.
			On("ByID", s.initialSealedResult.ID()).
			Return(s.initialSealedResult, nil).
			Once()

		s.headers.
			On("ByHeight", s.initialSealedHeader.Height).
			Return(nil, storage.ErrNotFound).
			Once()

		forest, err := ingestion2.NewResultsForest(
			s.log,
			s.headers,
			s.executionResults,
			s.pipelineFactory,
			s.latestPersistedSealedResult,
			s.maxViewDelta,
		)
		s.Error(err)
		s.Nil(forest)
	})
}

// TestResetLowestRejectedView tests the different scenarios of the ResetLowestRejectedView method,
// and verifies that the correct values for the last sealed view and rejected status are returned.
func (s *ResultsForestSuite) TestResetLowestRejectedView() {
	resultGen := s.g.ExecutionResults()

	s.Run("returns last sealed view and false when no results rejected", func() {
		// start with the latest persisted sealed result already inserted
		forest := s.createForest()

		view, rejected := forest.ResetLowestRejectedView()

		s.False(rejected)
		s.Equal(s.initialSealedHeader.View, view)
	})

	s.Run("returns true when results have been rejected", func() {
		// start with the latest persisted sealed result already inserted
		forest := s.createForest()

		// insert the first result that extends the latest persisted sealed result;
		// this should never be rejected irrespective of its view, because it is the direct child of the
		pipeline := s.createPipeline(s.results[0])
		pipeline.On("SetSealed").Return().Once()

		err := forest.AddSealedResult(s.results[0])
		s.Require().NoError(err)

		// Add a result that exceeds maxViewDelta to trigger rejection
		futureView := s.initialSealedHeader.View + s.maxViewDelta + 1
		futureResult := resultGen.Fixture(resultGen.WithBlock(s.blockWithView(futureView)))

		// pipeline optimistically created even when it's not added to the forest, but since it's not
		// added, SetSealed is not called
		_ = s.createPipeline(futureResult)

		err = forest.AddSealedResult(futureResult)
		s.Require().ErrorIs(err, ingestion2.ErrMaxViewDeltaExceeded)

		// Now reset should return true for rejected
		view, rejected := forest.ResetLowestRejectedView()
		s.Equal(s.blocks[0].View, view)
		s.True(rejected)

		// Second call should return false
		view, rejected = forest.ResetLowestRejectedView()
		s.Equal(s.blocks[0].View, view)
		s.False(rejected)
	})
}

// TestAddSealedResult tests adding sealed execution results to the forest.
// It covers successful addition, error cases, and invariant enforcement.
func (s *ResultsForestSuite) TestAddSealedResult() {
	resultGen := s.g.ExecutionResults()
	receiptGen := s.g.ExecutionReceipts()

	s.Run("add result that extends an existing sealed result", func() {
		forest := s.createForest()

		// insert chain of sealed results in order
		for i, result := range s.results {
			executedBlock := s.blocks[i]
			pipeline := s.createPipeline(result)
			pipeline.On("SetSealed").Return().Once()

			err := forest.AddSealedResult(result)
			s.Require().NoError(err)

			s.assertContainer(forest, result.ID(), ingestion2.ResultSealed)
			latestSealedView, latestSealedResult := forest.GetSealingProgress()
			s.Require().Equal(result, latestSealedResult)
			s.Require().Equal(executedBlock.View, latestSealedView)
		}

		// all results plus the latest persisted sealed result should be in the forest
		s.Equal(uint(len(s.results)+1), forest.Size())
	})

	s.Run("adding sealed result multiple times is idempotent", func() {
		forest := s.createForest()

		pipeline := s.createPipeline(s.results[0])
		pipeline.On("SetSealed").Return().Once()

		err := forest.AddSealedResult(s.results[0])
		s.Require().NoError(err)

		err = forest.AddSealedResult(s.results[0])
		s.Require().NoError(err)

		err = forest.AddSealedResult(s.results[0])
		s.Require().NoError(err)

		s.Equal(uint(2), forest.Size())
	})

	s.Run("adding sealed result with view lower than the forest's internal pruning threshold results in dedicated sentinel error", func() {
		forest := s.createForest()

		// create a result with view that's lower than the latest persisted sealed result
		view := s.initialSealedHeader.View - 1
		lowResult := resultGen.Fixture(resultGen.WithBlock(s.blockWithView(view)))

		_ = s.createPipeline(lowResult) // allow forest to (optimistically) construct a processing pipeline

		err := forest.AddSealedResult(lowResult)
		s.ErrorIs(err, ingestion2.ErrPrunedView)

		// only the latest persisted sealed result should be in the forest
		s.Equal(uint(1), forest.Size())
	})

	s.Run("adding result causes siblings to be abandoned", func() {
		forest := s.createForest()

		// create an additional result for block[0] with the same parent.
		conflictingResult := resultGen.Fixture(
			resultGen.WithBlock(s.blocks[0]),
			resultGen.WithPreviousResultID(s.initialSealedResult.ID()),
		)
		conflictingReceipt := receiptGen.Fixture(
			receiptGen.WithExecutionResult(*conflictingResult),
		)

		// add it to the forest as an unsealed result
		conflictingPipeline := s.createPipeline(conflictingResult)

		added, err := forest.AddReceipt(conflictingReceipt, ingestion2.ResultForCertifiedBlock)
		s.Require().NoError(err)
		s.True(added)

		// the forest should now contain the new unsealed result
		s.assertContainer(forest, conflictingResult.ID(), ingestion2.ResultForCertifiedBlock)
		s.Equal(uint(2), forest.Size())

		// now, add the sealed result for the same block
		sealedPipeline := s.createPipeline(s.results[0])
		sealedPipeline.On("SetSealed").Return().Once()

		// the conflicting pipeline should be marked abandoned once the sealed result is added
		conflictingPipeline.On("Abandon").Return().Once()

		err = forest.AddSealedResult(s.results[0])
		s.Require().NoError(err)

		// the forest should now contain the new sealed result, and the unsealed result should be abandoned
		s.assertContainer(forest, s.results[0].ID(), ingestion2.ResultSealed)
		s.assertContainer(forest, conflictingResult.ID(), ingestion2.ResultForCertifiedBlock)
		s.Equal(uint(3), forest.Size())
	})

	s.Run("rejects result that does not extend an existing sealed result", func() {
		forest := s.createForest()

		// this will be rejected, so SetSealed is not called
		_ = s.createPipeline(s.results[1])

		err := forest.AddSealedResult(s.results[1])
		s.Require().ErrorContains(err, "does not extend last sealed result")

		s.Equal(uint(1), forest.Size())
	})

	s.Run("rejects result with view higher than the view horizon", func() {
		forest := s.createForest()

		// create a result that descends from the latest persisted sealed result and has a view
		// higher than the view horizon
		highResult := resultGen.Fixture(
			resultGen.WithBlock(s.blockWithView(s.initialSealedHeader.View+s.maxViewDelta+1)),
			resultGen.WithPreviousResultID(s.initialSealedResult.ID()),
		)

		_ = s.createPipeline(highResult)

		err := forest.AddSealedResult(highResult)
		s.Require().ErrorIs(err, ingestion2.ErrMaxViewDeltaExceeded)

		// only the latest persisted sealed result should be in the forest
		s.Equal(uint(1), forest.Size())
	})

	s.Run("allows result with view higher than the view horizon if the parent is the latest persisted sealed result", func() {
		forest := s.createForest()

		// create a result that descends from the latest persisted sealed result and whose parent view
		// is also the latest persisted sealed result, but whose view is higher than the view horizon
		blockGen := s.g.Blocks()
		resultGen := s.g.ExecutionResults()

		view := s.initialSealedHeader.View + s.maxViewDelta + 1
		block := blockGen.Fixture(
			blockGen.WithParentView(s.initialSealedHeader.View),
			blockGen.WithView(view),
		)
		s.allBlocks[block.ID()] = block

		highResult := resultGen.Fixture(
			resultGen.WithBlock(block),
			resultGen.WithPreviousResultID(s.initialSealedResult.ID()),
		)

		pipeline := s.createPipeline(highResult)
		pipeline.On("SetSealed").Return().Once()

		// result should be added to the forest
		err := forest.AddSealedResult(highResult)
		s.NoError(err)
		s.Equal(uint(2), forest.Size())
	})

	s.Run("rejects result with unknown block", func() {
		forest := s.createForest()

		unknownResult := resultGen.Fixture()

		// no pipeline should be created

		err := forest.AddSealedResult(unknownResult)
		s.Require().ErrorContains(err, "failed to get block header for result")

		// only the latest persisted sealed result should be in the forest
		s.Equal(uint(1), forest.Size())
	})

	s.Run("rejects result that breaks forest invariant", func() {
		forest := s.createForest()

		sealedResult := s.results[0]

		// create a result with the same PreviousResultID, but different parent view
		incorrectResult := resultGen.Fixture(
			resultGen.WithBlock(s.blocks[1]),
			resultGen.WithPreviousResultID(sealedResult.PreviousResultID),
		)

		// first add a result that extends the latest persisted sealed result
		pipeline := s.createPipeline(sealedResult)
		pipeline.On("SetSealed").Return().Once()

		err := forest.AddSealedResult(sealedResult)
		s.Require().NoError(err)

		// next, add a result with the same parent, but different parent view
		_ = s.createPipeline(incorrectResult)

		err = forest.AddSealedResult(incorrectResult)
		s.Error(err)
		s.True(forestmodule.IsInvalidVertexError(err))

		// only the latest persisted sealed result and the result first result should be in the forest
		s.Equal(uint(2), forest.Size())
	})
}

// TestAddSealedResult_ConcurrentWithAddReceipt tests the concurrent calls to AddSealedResult and
// AddReceipt are handled as expected.
//
// This test creates a main fork of sealed results and execution forks with up to 3 results
// descending from each sealed result. Then it concurrently calls AddSealedResult for all sealed
// results and AddReceipt for all conflicting results, and verifies the final state of the forest.
func (s *ResultsForestSuite) TestAddSealedResult_ConcurrentWithAddReceipt() {
	blockGen := s.g.Blocks()
	receiptGen := s.g.ExecutionReceipts()

	forest := s.createForest()

	// with 50 blocks, there should be ~200 results in the forest
	blocks := blockGen.List(50, blockGen.WithParentHeader(s.initialSealedHeader))
	for _, block := range blocks {
		s.allBlocks[block.ID()] = block
	}

	sealedResults := s.executionFork(blocks, s.initialSealedResult.ID())
	conflictingReceipts := make([]*flow.ExecutionReceipt, 0)
	for i, result := range sealedResults {
		sealedPipeline := s.createPipeline(result)
		sealedPipeline.On("SetSealed").Return().Once()
		sealedPipeline.On("GetState").Return(optimistic_sync.StatePending).Maybe()

		// create conflicting forks of up to 3 results descending from the parent of the sealed result
		forkBlocks := blocks[i:min(i+3, len(blocks))]
		forkResults := s.executionFork(forkBlocks, result.PreviousResultID)

		// preconfigure the pipelines and results so insertions happen concurrently
		for _, r := range forkResults {
			// pipelines should return pending until they are abandoned. they will all be abandoned
			// on insertion since they are either siblings of the sealed results, or descend from an
			// abandoned parent.
			isAbandoned := atomic.NewBool(false)
			forkPipeline := s.createPipeline(r)
			forkPipeline.On("GetState").Return(func() optimistic_sync.State {
				if isAbandoned.Load() {
					return optimistic_sync.StateAbandoned
				}
				return optimistic_sync.StatePending
			}).Maybe()
			forkPipeline.On("Abandon").Return().Run(func(args mock.Arguments) {
				isAbandoned.Store(true)
			})

			receipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*result))
			conflictingReceipts = append(conflictingReceipts, receipt)
		}
	}

	// concurrently insert sealed results with AddSealedResult and conflicting receipts with AddReceipt
	// AddSealedResult must be called sequentially to ensure the forest invariant is maintained and
	// the results are actually added to the forest.
	// synchronize goroutine startup to maximize concurrency
	synctest.Test(s.T(), func(t *testing.T) {
		start := make(chan struct{})
		for _, receipt := range conflictingReceipts {
			go func() {
				<-start
				added, err := forest.AddReceipt(receipt, ingestion2.ResultForCertifiedBlock)
				s.Require().NoError(err)
				s.True(added)
			}()
		}

		// wait until all goroutines are running, then release them all at once
		synctest.Wait()
		close(start)

		for _, result := range sealedResults {
			err := forest.AddSealedResult(result)
			s.Require().NoError(err)
		}

		synctest.Wait()
	})

	// there should be one result for each of the sealed and conflicting results, plus the latest
	// persisted sealed result. All sealed results should be marked as sealed, and conflicting results
	// should have the correct receipt and be marked Abandoned.
	expectedSize := uint(1 + len(sealedResults) + len(conflictingReceipts))
	s.Equal(expectedSize, forest.Size())

	for _, result := range sealedResults {
		s.assertContainer(forest, result.ID(), ingestion2.ResultSealed)
	}
	for _, receipt := range conflictingReceipts {
		resultID := receipt.ExecutionResult.ID()
		container, found := forest.GetContainer(resultID)
		s.True(found)
		s.Equal(resultID, container.ResultID())
		s.Equal(uint(1), container.Size())
		s.True(container.Has(receipt.ID()))
		s.Equal(ingestion2.ResultForCertifiedBlock, container.ResultStatus())

		// this is mocked, but should be abandoned for all conflicting results
		s.Equal(optimistic_sync.StateAbandoned, container.Pipeline().GetState())
	}
}

// TestAddReceipt tests adding execution receipts to the forest.
// It covers successful addition, duplicate handling, and error cases.
func (s *ResultsForestSuite) TestAddReceipt() {
	resultGen := s.g.ExecutionResults()
	receiptGen := s.g.ExecutionReceipts()

	s.Run("add receipt for new result that extends an existing result", func() {
		forest := s.createForest()

		result := s.results[0]
		receipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*result))

		pipeline := s.createPipeline(result)
		pipeline.On("SetSealed").Return().Once()

		added, err := forest.AddReceipt(receipt, ingestion2.ResultSealed)
		s.Require().NoError(err)
		s.True(added)

		s.assertContainer(forest, result.ID(), ingestion2.ResultSealed)
		s.Equal(uint(2), forest.Size())
	})

	s.Run("add receipt for new result that does not extend an existing result", func() {
		forest := s.createForest()

		result := s.results[1]
		receipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*result))

		_ = s.createPipeline(result)

		added, err := forest.AddReceipt(receipt, ingestion2.ResultForCertifiedBlock)
		s.Require().NoError(err)
		s.True(added)

		s.assertContainer(forest, result.ID(), ingestion2.ResultForCertifiedBlock)
		s.Equal(uint(2), forest.Size())
	})

	s.Run("add receipt for existing result", func() {
		forest := s.createForest()

		result := s.results[0]
		receipt1 := receiptGen.Fixture(receiptGen.WithExecutionResult(*result))
		receipt2 := receiptGen.Fixture(receiptGen.WithExecutionResult(*result))

		_ = s.createPipeline(result)

		// add the first receipt
		added, err := forest.AddReceipt(receipt1, ingestion2.ResultForCertifiedBlock)
		s.Require().NoError(err)
		s.True(added)

		// result should be added to the forest
		s.assertContainer(forest, result.ID(), ingestion2.ResultForCertifiedBlock)
		s.Equal(uint(2), forest.Size())

		// add the second receipt
		added, err = forest.AddReceipt(receipt2, ingestion2.ResultForCertifiedBlock)
		s.Require().NoError(err)
		s.True(added)
	})

	s.Run("add receipt for finalized result with view lower that finalized view", func() {
		forest := s.createForest()

		result := s.results[0]
		receipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*result))

		// progress finalized view to block[0]
		err := forest.OnBlockFinalized(s.blocks[0])
		s.Require().NoError(err)

		_ = s.createPipeline(result)

		added, err := forest.AddReceipt(receipt, ingestion2.ResultForFinalizedBlock)
		s.Require().NoError(err)
		s.True(added)

		s.assertContainer(forest, result.ID(), ingestion2.ResultForFinalizedBlock)
		s.Equal(uint(2), forest.Size())
	})

	s.Run("add receipt for finalized result with view higher that finalized view does not set finalized status", func() {
		forest := s.createForest()

		result := s.results[0]
		receipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*result))

		_ = s.createPipeline(result)

		added, err := forest.AddReceipt(receipt, ingestion2.ResultForFinalizedBlock)
		s.Require().NoError(err)
		s.True(added)

		s.assertContainer(forest, result.ID(), ingestion2.ResultForCertifiedBlock)
		s.Equal(uint(2), forest.Size())
	})

	s.Run("add receipt multiple times is idempotent", func() {
		forest := s.createForest()

		result := s.results[0]
		receipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*result))

		_ = s.createPipeline(result)

		added, err := forest.AddReceipt(receipt, ingestion2.ResultForCertifiedBlock)
		s.Require().NoError(err)
		s.True(added)

		s.assertContainer(forest, result.ID(), ingestion2.ResultForCertifiedBlock)
		s.Equal(uint(2), forest.Size())

		added, err = forest.AddReceipt(receipt, ingestion2.ResultForCertifiedBlock)
		s.Require().NoError(err)
		s.False(added)
		s.Equal(uint(2), forest.Size())
	})

	s.Run("add receipt for result with view lower than the latest persisted sealed result is a no-op", func() {
		forest := s.createForest()

		view := s.initialSealedHeader.View - 1
		result := resultGen.Fixture(resultGen.WithBlock(s.blockWithView(view)))
		receipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*result))

		_ = s.createPipeline(result)

		added, err := forest.AddReceipt(receipt, ingestion2.ResultForCertifiedBlock)
		s.ErrorIs(err, ingestion2.ErrPrunedView)
		s.False(added)
		s.Equal(uint(1), forest.Size())
	})

	s.Run("adding result that extends an abandoned fork abandons result", func() {
		forest := s.createForest()

		abandonedResult := s.results[0]
		abandonedReceipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*abandonedResult))

		abandonedPipeline := s.createPipeline(abandonedResult)

		added, err := forest.AddReceipt(abandonedReceipt, ingestion2.ResultForCertifiedBlock)
		s.Require().NoError(err)
		s.True(added)

		result := s.results[1]
		receipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*result))

		pipeline := s.createPipeline(result)
		pipeline.On("Abandon").Return().Once()

		// simulate the parent already being abandoned
		abandonedPipeline.On("GetState").Return(optimistic_sync.StateAbandoned).Once()

		added, err = forest.AddReceipt(receipt, ingestion2.ResultForCertifiedBlock)
		s.Require().NoError(err)
		s.True(added)

		s.Equal(uint(3), forest.Size())
	})

	s.Run("adding result for abandoned fork abandons result and all its descendants", func() {
		forest := s.createForest()

		// create an execution fork of 3 results descending from the latest persisted sealed result
		conflictingFork := make([]*flow.ExecutionResult, 3)
		prevResultID := s.initialSealedResult.ID()
		for i := range conflictingFork {
			conflictingFork[i] = resultGen.Fixture(
				resultGen.WithBlock(s.blocks[i]),
				resultGen.WithPreviousResultID(prevResultID),
			)
			prevResultID = conflictingFork[i].ID()
		}

		// 1. add the second result. since its parent does not exist yet, it will be added and
		// NOT abandoned.
		receipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*conflictingFork[1]))
		conflictingPipeline1 := s.createPipeline(conflictingFork[1])

		added, err := forest.AddReceipt(receipt, ingestion2.ResultForCertifiedBlock)
		s.Require().NoError(err)
		s.True(added)

		// 2. add a descendant result which will also be abandoned
		receipt = receiptGen.Fixture(receiptGen.WithExecutionResult(*conflictingFork[2]))
		conflictingPipeline2 := s.createPipeline(conflictingFork[2])

		// this is called by isAbandonedFork when adding the descendant result
		// no results should be abandoned yet.
		conflictingPipeline1.On("GetState").Return(optimistic_sync.StatePending).Once()

		added, err = forest.AddReceipt(receipt, ingestion2.ResultForCertifiedBlock)
		s.Require().NoError(err)
		s.True(added)

		// 3. add a sealed result which will be a sibling to the first conflicting result
		// No results should be abandoned yet.
		sealedPipeline := s.createPipeline(s.results[0])
		sealedPipeline.On("SetSealed").Return().Once()

		err = forest.AddSealedResult(s.results[0])
		s.Require().NoError(err)

		// 4. the first conflicting result. It and all of its descendants should be abandoned
		receipt = receiptGen.Fixture(receiptGen.WithExecutionResult(*conflictingFork[0]))
		conflictingPipeline0 := s.createPipeline(conflictingFork[0])

		conflictingPipeline0.On("Abandon").Return().Once()
		conflictingPipeline1.On("Abandon").Return().Once()
		conflictingPipeline2.On("Abandon").Return().Once()

		added, err = forest.AddReceipt(receipt, ingestion2.ResultForCertifiedBlock)
		s.Require().NoError(err)
		s.True(added)
		s.Equal(uint(5), forest.Size()) // last persisted, sealed result, + 3 conflicting results
	})

	s.Run("rejects sealed result that does not extend an existing sealed result", func() {
		forest := s.createForest()

		result := s.results[1]
		receipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*result))

		_ = s.createPipeline(result)

		added, err := forest.AddReceipt(receipt, ingestion2.ResultSealed)
		s.ErrorContains(err, "does not extend last sealed result")
		s.False(added)
		s.Equal(uint(1), forest.Size())
	})

	s.Run("rejects result with view higher than the view horizon", func() {
		forest := s.createForest()

		view := s.initialSealedHeader.View + s.maxViewDelta + 1
		result := resultGen.Fixture(resultGen.WithBlock(s.blockWithView(view)))
		receipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*result))

		_ = s.createPipeline(result)

		added, err := forest.AddReceipt(receipt, ingestion2.ResultForCertifiedBlock)
		s.ErrorIs(err, ingestion2.ErrMaxViewDeltaExceeded)
		s.False(added)
		s.Equal(uint(1), forest.Size())
	})

	s.Run("rejects result with unknown block", func() {
		forest := s.createForest()

		unknownResult := resultGen.Fixture()
		receipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*unknownResult))

		added, err := forest.AddReceipt(receipt, ingestion2.ResultForCertifiedBlock)
		s.ErrorContains(err, "failed to get block header for result")
		s.False(added)
		s.Equal(uint(1), forest.Size())
	})

	s.Run("rejects result that breaks forest invariant", func() {
		forest := s.createForest()

		sealedResult := s.results[0]

		// create a result with the same PreviousResultID, but different parent view
		incorrectResult := resultGen.Fixture(
			resultGen.WithBlock(s.blocks[1]),
			resultGen.WithPreviousResultID(sealedResult.PreviousResultID),
		)
		incorrectReceipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*incorrectResult))

		// first add a result that extends the latest persisted sealed result
		pipeline := s.createPipeline(sealedResult)
		pipeline.On("SetSealed").Return().Once()

		err := forest.AddSealedResult(sealedResult)
		s.Require().NoError(err)

		// next, add a result with the same parent, but different parent view
		_ = s.createPipeline(incorrectResult)

		added, err := forest.AddReceipt(incorrectReceipt, ingestion2.ResultForCertifiedBlock)
		s.Error(err)
		s.True(forestmodule.IsInvalidVertexError(err))
		s.False(added)

		// only the latest persisted sealed result and the result first result should be in the forest
		s.Equal(uint(2), forest.Size())
	})

	s.Run("rejects invalid result status", func() {
		forest := s.createForest()

		added, err := forest.AddReceipt(receiptGen.Fixture(), ingestion2.ResultStatus(0))
		s.ErrorContains(err, "invalid result status")
		s.False(added)
		s.Equal(uint(1), forest.Size())
	})
}

// TestAddReceipt_ConcurrentInserts tests the concurrent calls to AddReceipt are handled as expected
func (s *ResultsForestSuite) TestAddReceipt_ConcurrentInserts() {
	receiptGen := s.g.ExecutionReceipts()

	forest := s.createForest()

	// NewPipeline will be called at least once for each result. it may be called more than once
	// depending on timing.
	for _, result := range s.results {
		pipeline := opsyncmock.NewPipeline(s.T())
		pipeline.On("GetState").Return(optimistic_sync.StatePending).Maybe()
		s.pipelineFactory.
			On("NewPipeline", result).
			Return(pipeline)
	}

	receiptsPerResult := 100

	// mark the first 2 blocks as finalized so we can use different result statuses
	for _, block := range s.blocks[:2] {
		err := forest.OnBlockFinalized(block)
		s.Require().NoError(err)
	}

	wg := sync.WaitGroup{}
	for i := range len(s.results) * receiptsPerResult {
		wg.Go(func() {
			index := i % len(s.results)
			status := ingestion2.ResultForCertifiedBlock
			if index < 2 {
				status = ingestion2.ResultForFinalizedBlock
			}
			receipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*s.results[index]))
			added, err := forest.AddReceipt(receipt, status)
			s.Require().NoError(err)
			s.True(added)
		})
	}
	wg.Wait()

	// verify all of the results were added and have the correct number of receipts
	for i, result := range s.results {
		container, found := forest.GetContainer(result.ID())
		s.True(found)
		s.Equal(result.ID(), container.ResultID())
		s.Equal(uint(receiptsPerResult), container.Size())
		if i < 2 {
			s.Equal(ingestion2.ResultForFinalizedBlock, container.ResultStatus())
		} else {
			s.Equal(ingestion2.ResultForCertifiedBlock, container.ResultStatus())
		}
	}
}

func (s *ResultsForestSuite) TestHasReceipt() {
	receiptGen := s.g.ExecutionReceipts()

	forest := s.createForest()

	result := s.results[0]
	receipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*result))

	// receipt should not be in the forest initially
	has := forest.HasReceipt(receipt)
	s.False(has)

	_ = s.createPipeline(result)
	added, err := forest.AddReceipt(receipt, ingestion2.ResultForCertifiedBlock)
	s.Require().NoError(err)
	s.True(added)

	// now it should be in the forest
	has = forest.HasReceipt(receipt)
	s.True(has)
}

// TestOnBlockFinalized tests the OnBlockFinalized handler by creating a forest with execution and
// consensus forks, then verifying the forks are abandoned as expected.
//
// This test works with 4 forks:
// - main chain
// - execution fork (2 results with same blocks)
// - 2 consensus forks (different blocks and results)
//
// It them walks through different scenarios of finalizing the blocks to test the forest's behavior.
func (s *ResultsForestSuite) TestOnBlockFinalized() {
	lastSealedResult := s.results[0]
	mainResults := s.results[1:]
	execFork := s.executionFork(s.blocks[1:], lastSealedResult.ID())
	consensusFork1 := s.consensusFork(2, s.blocks[0].ToHeader(), lastSealedResult.ID())
	consensusFork2 := s.consensusFork(2, s.blocks[0].ToHeader(), lastSealedResult.ID())

	// create forest and insert a sealed result
	setupForest := func() *ingestion2.ResultsForest {
		forest := s.createForest()
		sealedPipeline := s.createPipeline(lastSealedResult)
		sealedPipeline.On("SetSealed").Return().Once()
		sealedPipeline.On("GetState").Return(optimistic_sync.StatePending).Maybe()
		err := forest.AddSealedResult(lastSealedResult)
		s.Require().NoError(err)
		return forest
	}

	s.Run("marks results finalized and abandons conflicting forks", func() {
		forest := setupForest()

		_ = s.addToForest(forest, mainResults...)
		_ = s.addToForest(forest, execFork...)
		consensusForkPipelines := s.addToForest(forest, append(consensusFork1, consensusFork2...)...)

		// finalize the second block, abandons the 2 conflicting consensus forks, and marks the first
		// results in mainResults and execFork as finalized
		for _, pipeline := range consensusForkPipelines {
			pipeline.On("Abandon").Return().Once()
		}

		err := forest.OnBlockFinalized(s.blocks[1])
		s.Require().NoError(err)

		s.assertContainer(forest, mainResults[0].ID(), ingestion2.ResultForFinalizedBlock)
		s.assertContainer(forest, mainResults[1].ID(), ingestion2.ResultForCertifiedBlock)
		s.assertContainer(forest, execFork[0].ID(), ingestion2.ResultForFinalizedBlock)
		s.assertContainer(forest, execFork[1].ID(), ingestion2.ResultForCertifiedBlock)
	})

	s.Run("returns an error if sibling with same view is marked sealed", func() {
		forest := setupForest()

		pipelines := s.addToForest(forest, mainResults[0], execFork[0])

		// manually mark the first main result as sealed. this will avoid incrementing the `lastSealedView`
		// and leave the forest in an inconsistent state where there is an unexpected sealed result.
		pipelines[0].On("SetSealed").Return().Once()

		container, found := forest.GetContainer(mainResults[0].ID())
		s.True(found)
		err := container.SetResultStatus(ingestion2.ResultSealed)
		s.Require().NoError(err)

		err = forest.OnBlockFinalized(s.blocks[1])
		s.ErrorContains(err, "invalid result status transition: sealed -> finalized")
	})

	s.Run("returns an error if sibling with different view is already marked as finalized", func() {
		forest := setupForest()

		_ = s.addToForest(forest, mainResults[0], consensusFork1[0])

		container, found := forest.GetContainer(consensusFork1[0].ID())
		s.True(found)
		err := container.SetResultStatus(ingestion2.ResultForFinalizedBlock)
		s.Require().NoError(err)

		err = forest.OnBlockFinalized(s.blocks[1])
		s.ErrorContains(err, "is different from the latest finalized block")
	})

	s.Run("marks sealed results as sealed, and abandons conflicting forks", func() {
		// create the forest WITHOUT adding the first sealed result
		forest := s.createForest()

		mainPipelines := s.addToForest(forest, s.results...)
		forkPipelines := s.addToForest(forest, execFork...)

		err := forest.OnBlockFinalized(s.blocks[0])
		s.Require().NoError(err)

		// Block 0: Finalized
		s.assertContainer(forest, s.results[0].ID(), ingestion2.ResultForFinalizedBlock) // block 0
		s.assertContainer(forest, s.results[1].ID(), ingestion2.ResultForCertifiedBlock) // block 1
		s.assertContainer(forest, s.results[2].ID(), ingestion2.ResultForCertifiedBlock) // block 2
		s.assertContainer(forest, s.results[3].ID(), ingestion2.ResultForCertifiedBlock) // block 3
		s.assertContainer(forest, execFork[0].ID(), ingestion2.ResultForCertifiedBlock)  // block 1
		s.assertContainer(forest, execFork[1].ID(), ingestion2.ResultForCertifiedBlock)  // block 2

		err = forest.OnBlockFinalized(s.blocks[1])
		s.Require().NoError(err)

		// Block 0 & 1: Finalized
		s.assertContainer(forest, s.results[0].ID(), ingestion2.ResultForFinalizedBlock) // block 0
		s.assertContainer(forest, s.results[1].ID(), ingestion2.ResultForFinalizedBlock) // block 1
		s.assertContainer(forest, s.results[2].ID(), ingestion2.ResultForCertifiedBlock) // block 2
		s.assertContainer(forest, s.results[3].ID(), ingestion2.ResultForCertifiedBlock) // block 3
		s.assertContainer(forest, execFork[0].ID(), ingestion2.ResultForFinalizedBlock)  // block 1
		s.assertContainer(forest, execFork[1].ID(), ingestion2.ResultForCertifiedBlock)  // block 2

		// block 2 contains a seal for the first main result
		mainPipelines[0].On("SetSealed").Return().Once()

		err = forest.OnBlockFinalized(s.blocks[2])
		s.Require().NoError(err)

		// Result 0: Sealed
		// Block 1 & 2: Finalized
		s.assertContainer(forest, s.results[0].ID(), ingestion2.ResultSealed)            // block 0
		s.assertContainer(forest, s.results[1].ID(), ingestion2.ResultForFinalizedBlock) // block 1
		s.assertContainer(forest, s.results[2].ID(), ingestion2.ResultForFinalizedBlock) // block 2
		s.assertContainer(forest, s.results[3].ID(), ingestion2.ResultForCertifiedBlock) // block 3
		s.assertContainer(forest, execFork[0].ID(), ingestion2.ResultForFinalizedBlock)  // block 1
		s.assertContainer(forest, execFork[1].ID(), ingestion2.ResultForFinalizedBlock)  // block 2

		// block 3 contains a seal for the second main result
		mainPipelines[1].On("SetSealed").Return().Once()

		// the execution fork conflicts with the second main result, so it should be abandoned after
		// finalizing block 3
		for _, pipeline := range forkPipelines {
			pipeline.On("Abandon").Return().Once()
		}

		err = forest.OnBlockFinalized(s.blocks[3])
		s.Require().NoError(err)

		// Result 0 & 1: Sealed
		// Block 2 & 3: Finalized
		s.assertContainer(forest, s.results[0].ID(), ingestion2.ResultSealed)            // block 0
		s.assertContainer(forest, s.results[1].ID(), ingestion2.ResultSealed)            // block 1
		s.assertContainer(forest, s.results[2].ID(), ingestion2.ResultForFinalizedBlock) // block 2
		s.assertContainer(forest, s.results[3].ID(), ingestion2.ResultForFinalizedBlock) // block 3
	})
}

// TestOnBlockFinalized_CornerCases tests OnBlockFinalized under corner case scenarios:
// - missing sealed results
// - missing sealed results when rejectedResults is true
// - out of order seals
// - duplicate on finalized notifications are skipped
// - on finalized notifications that lag sealed results are skipped
func (s *ResultsForestSuite) TestOnBlockFinalized_CornerCases() {

	blockGen := s.g.Blocks()
	resultGen := s.g.ExecutionResults()
	receiptGen := s.g.ExecutionReceipts()
	payloadGen := s.g.Payloads()
	sealGen := s.g.Seals()

	// create a block with a seal for the second result, skipping the first
	finalizedBlockWithSkippedSeal := blockGen.Fixture(
		blockGen.WithParentHeader(s.blocks[len(s.blocks)-1].ToHeader()),
		blockGen.WithPayload(payloadGen.Fixture(
			payloadGen.WithSeals(
				sealGen.Fixture(sealGen.WithResultID(s.results[1].ID())),
			),
		)),
	)

	s.Run("returns an error if a seal is missing", func() {
		forest := s.createForest()
		_ = s.addToForest(forest, s.results...)
		// SetSealed will not be called

		err := forest.OnBlockFinalized(finalizedBlockWithSkippedSeal)
		s.ErrorContains(err, "parent result not found in forest or is not sealed")
	})

	s.Run("skips processing seals if a seal is missing and rejectedResults is true", func() {
		// create a result with a view outside of the view horizon.
		view := s.initialSealedHeader.View + s.maxViewDelta + 1
		rejectedResult := resultGen.Fixture(resultGen.WithBlock(s.blockWithView(view)))
		rejectedReceipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*rejectedResult))

		forest := s.createForest()
		_ = s.addToForest(forest, s.results...)

		// the forest will reject the result and set the rejectedResults flag
		_ = s.createPipeline(rejectedResult)
		added, err := forest.AddReceipt(rejectedReceipt, ingestion2.ResultForCertifiedBlock)
		s.ErrorIs(err, ingestion2.ErrMaxViewDeltaExceeded)
		s.False(added)

		// finalizing the block with the skipped seal should not cause an error because the forest
		// has rejected a result.
		err = forest.OnBlockFinalized(finalizedBlockWithSkippedSeal)
		s.NoError(err)

		s.assertContainer(forest, s.results[0].ID(), ingestion2.ResultForCertifiedBlock)
		s.assertContainer(forest, s.results[1].ID(), ingestion2.ResultForCertifiedBlock)
	})

	s.Run("sorts seals by view", func() {
		// create a new block with seals for all of s.results, in random order
		finalizedBlock := blockGen.Fixture(
			blockGen.WithParentHeader(s.blocks[len(s.blocks)-1].ToHeader()),
			blockGen.WithPayload(payloadGen.Fixture(
				payloadGen.WithSeals(
					sealGen.Fixture(sealGen.WithResultID(s.results[3].ID())),
					sealGen.Fixture(sealGen.WithResultID(s.results[0].ID())),
					sealGen.Fixture(sealGen.WithResultID(s.results[2].ID())),
					sealGen.Fixture(sealGen.WithResultID(s.results[1].ID())),
				),
			)),
		)

		forest := s.createForest()
		pipelines := s.addToForest(forest, s.results...)
		for _, pipeline := range pipelines {
			pipeline.On("SetSealed").Return().Once()
		}

		// finalize the block. if the seals are not sorted, there will be a gap between the last
		// sealed result and the next, which will cause the method to return an error.
		err := forest.OnBlockFinalized(finalizedBlock)
		s.Require().NoError(err)

		s.assertContainer(forest, s.results[0].ID(), ingestion2.ResultSealed)
		s.assertContainer(forest, s.results[1].ID(), ingestion2.ResultSealed)
		s.assertContainer(forest, s.results[2].ID(), ingestion2.ResultSealed)
		s.assertContainer(forest, s.results[3].ID(), ingestion2.ResultSealed)
	})

	s.Run("skips notification if finalized view was already processed", func() {
		execFork := s.executionFork(s.blocks[0:1], s.initialSealedResult.ID())

		forest := s.createForest()
		_ = s.addToForest(forest, s.results...)

		err := forest.OnBlockFinalized(s.blocks[0])
		s.Require().NoError(err)

		// add a new result with non-finalized status and run OnBlockFinalized again.
		// note: during normal operation, the logic that adds results into the forest should indicate
		// the result was finalized during insertion. here we are inserting it as certified.
		_ = s.addToForest(forest, execFork...)

		err = forest.OnBlockFinalized(s.blocks[0])
		s.Require().NoError(err)

		// the new block should not be marked finalized
		s.assertContainer(forest, s.results[0].ID(), ingestion2.ResultForFinalizedBlock)
		s.assertContainer(forest, s.results[1].ID(), ingestion2.ResultForCertifiedBlock)
		s.assertContainer(forest, execFork[0].ID(), ingestion2.ResultForCertifiedBlock)
	})

	s.Run("skips notification if finalized view is lower than or equal to last sealed view", func() {
		// create a block with a seal for result[1]
		finalizedBlockWithSeal := blockGen.Fixture(
			blockGen.WithParentHeader(s.initialSealedHeader),
			blockGen.WithPayload(payloadGen.Fixture(
				payloadGen.WithSeals(
					sealGen.Fixture(sealGen.WithResultID(s.results[1].ID())),
				),
			)),
		)

		forest := s.createForest()

		// add the result[0] as sealed. this will set the lastSealedView to block[0]
		sealedPipeline := s.createPipeline(s.results[0])
		sealedPipeline.On("SetSealed").Return().Once()
		sealedPipeline.On("GetState").Return(optimistic_sync.StatePending).Maybe()
		err := forest.AddSealedResult(s.results[0])
		s.Require().NoError(err)

		// add all of the results to the forest.
		_ = s.addToForest(forest, s.results[1:]...)

		// finalize the block with a seal for result[1]. if this block was processed, result[1] would
		// be marked as sealed.
		err = forest.OnBlockFinalized(finalizedBlockWithSeal)
		s.Require().NoError(err)

		// result[1] should NOT be marked as sealed
		s.assertContainer(forest, s.results[0].ID(), ingestion2.ResultSealed)
		s.assertContainer(forest, s.results[1].ID(), ingestion2.ResultForCertifiedBlock)
	})
}

// TestOnStateUpdated tests the OnStateUpdated method by setting up a parent result with 3 children,
// then sending state updates to simulate each of the unique cases:
//
// - Abandoned -> child updates should be skipped
// - Complete -> child updates should be sent and the forest should be pruned
// - All other states -> child updates should be sent
func (s *ResultsForestSuite) TestOnStateUpdated() {
	resultGen := s.g.ExecutionResults()
	receiptGen := s.g.ExecutionReceipts()

	forest := s.createForest()

	parentResult := s.results[0]
	parentID := parentResult.ID()
	parentView := s.blocks[0].View

	children := resultGen.List(3,
		resultGen.WithBlock(s.blocks[1]),
		resultGen.WithPreviousResultID(parentID),
	)

	// add parent result
	parentPipeline := s.createPipeline(parentResult)
	parentPipeline.On("SetSealed").Return().Once()

	err := forest.AddSealedResult(parentResult)
	s.Require().NoError(err)

	// add children
	childPipelines := make([]*opsyncmock.Pipeline, len(children))
	for i, result := range children {
		childPipelines[i] = s.createPipeline(result)
		receipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*result))

		// called by isAbandonedFork when adding the child result
		parentPipeline.On("GetState").Return(optimistic_sync.StatePending).Once()

		added, err := forest.AddReceipt(receipt, ingestion2.ResultForCertifiedBlock)
		s.Require().NoError(err)
		s.True(added)
	}
	s.Equal(s.initialSealedHeader.View, forest.LowestView())
	s.Equal(uint(5), forest.Size()) // last persisted, sealed result, + 2 children

	s.Run("sends state update to all children", func() {
		// all states except Abandoned and Complete
		states := []optimistic_sync.State{
			optimistic_sync.StatePending,
			optimistic_sync.StateProcessing,
			optimistic_sync.StateWaitingPersist,
		}
		for _, state := range states {
			// OnParentStateUpdated should be called on all children
			for _, childPipeline := range childPipelines {
				childPipeline.On("OnParentStateUpdated", state).Return().Once()
			}
			forest.OnStateUpdated(parentID, state)
			s.Equal(uint(5), forest.Size()) // the forest should not have changed
		}
	})

	s.Run("skips notifications if new state is abandoned", func() {
		// OnParentStateUpdated should not be called
		forest.OnStateUpdated(parentID, optimistic_sync.StateAbandoned)
		s.Equal(uint(5), forest.Size()) // the forest should not have changed
	})

	s.Run("sends state update to all children and prunes when complete", func() {
		// simulate the pipeline completing processing by updating the last persisted sealed result
		s.latestPersistedSealedResult.On("Latest").Return(parentID, parentView).Once()
		for _, childPipeline := range childPipelines {
			childPipeline.On("OnParentStateUpdated", optimistic_sync.StateComplete).Return().Once()
		}
		forest.OnStateUpdated(parentID, optimistic_sync.StateComplete)

		// after the pruning, the lowest view should be the first block view
		s.Equal(parentView, forest.LowestView())
		s.Equal(uint(4), forest.Size())
	})
}

// TestOnStateUpdated_ConcurrentUpdates tests the concurrent calls to OnStateUpdated are handled
// as expected without panics.
//
// The test creates a balanced tree of results with 5 descendants per vertex. Each result in the tree
// is added to the forest, then the test concurrently calls OnStateUpdated on each result cycling
// through the states `pending`, `processing`, and `waiting_persist`.
func (s *ResultsForestSuite) TestOnStateUpdated_ConcurrentUpdates() {
	receiptGen := s.g.ExecutionReceipts()

	// s.maxViewDelta = 1000 // blocklist may have non-sequential views
	forest := s.createForest()

	// this will generate a tree with 780 results
	allResults := s.balancedTree(5, s.initialSealedResult.ID(), s.blocks)

	// insert the results into the forest
	for _, result := range allResults {
		pipeline := s.createPipeline(result)
		pipeline.On("GetState").Return(optimistic_sync.StatePending).Maybe()
		pipeline.On("OnParentStateUpdated", mock.AnythingOfType("optimistic_sync.State")).Maybe()

		receipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*result))
		added, err := forest.AddReceipt(receipt, ingestion2.ResultForCertifiedBlock)
		s.Require().NoError(err)
		s.True(added)
	}

	states := []optimistic_sync.State{
		optimistic_sync.StatePending,
		optimistic_sync.StateProcessing,
		optimistic_sync.StateWaitingPersist,
	}

	// cycle all through each of the states
	wg := sync.WaitGroup{}
	for _, result := range allResults {
		wg.Go(func() {
			for _, state := range states {
				forest.OnStateUpdated(result.ID(), state)
			}
		})
	}
	wg.Wait()
}

// createForest creates a new ResultsForest with the latest persisted sealed result added to it.
func (s *ResultsForestSuite) createForest() *ingestion2.ResultsForest {
	// always called in the constructor
	s.latestPersistedSealedResult.
		On("Latest").
		Return(s.initialSealedResult.ID(), s.initialSealedHeader.Height).
		Once()
	s.executionResults.
		On("ByID", s.initialSealedResult.ID()).
		Return(s.initialSealedResult, nil).
		Once()
	s.headers.
		On("ByHeight", s.initialSealedHeader.Height).
		Return(s.initialSealedHeader, nil).
		Once()

	pipeline := opsyncmock.NewPipeline(s.T())
	pipeline.On("SetSealed").Return().Once()
	pipeline.On("GetState").Return(optimistic_sync.StateComplete).Maybe()
	s.pipelineFactory.
		On("NewCompletedPipeline", s.initialSealedResult).
		Return(pipeline).
		Once()

	forest, err := ingestion2.NewResultsForest(
		s.log,
		s.headers,
		s.executionResults,
		s.pipelineFactory,
		s.latestPersistedSealedResult,
		s.maxViewDelta,
	)
	s.Require().NoError(err)

	return forest
}

// assertContainer asserts that the container for the given result ID exists in the forest and has
// the provided result status.
func (s *ResultsForestSuite) assertContainer(
	forest *ingestion2.ResultsForest,
	resultID flow.Identifier,
	resultStatus ingestion2.ResultStatus,
) {
	container, found := forest.GetContainer(resultID)
	s.Require().True(found)
	s.Equal(resultStatus, container.ResultStatus())
	s.Equal(resultID, container.ResultID())
}

// createPipeline creates a new pipeline for the given result, and configures the pipeline factory to return it.
func (s *ResultsForestSuite) createPipeline(result *flow.ExecutionResult) *opsyncmock.Pipeline {
	pipeline := opsyncmock.NewPipeline(s.T())
	s.pipelineFactory.
		On("NewPipeline", result).
		Return(pipeline).
		Once()

	return pipeline
}

// blockWithView creates a new block with the given view, and sets the parent view to the previous view.
func (s *ResultsForestSuite) blockWithView(view uint64) *flow.Block {
	blockGen := s.g.Blocks()

	block := blockGen.Fixture(
		blockGen.WithParentView(view-1),
		blockGen.WithView(view),
	)
	s.allBlocks[block.ID()] = block

	return block
}

// executionFork creates a new chain of execution results for the provided blocks descending from
// the provided previous result ID.
func (s *ResultsForestSuite) executionFork(
	blocks []*flow.Block,
	prevResultID flow.Identifier,
) []*flow.ExecutionResult {
	resultGen := s.g.ExecutionResults()
	results := make([]*flow.ExecutionResult, len(blocks))
	for i, block := range blocks {
		results[i] = resultGen.Fixture(
			resultGen.WithBlock(block),
			resultGen.WithPreviousResultID(prevResultID),
		)
		prevResultID = results[i].ID()
	}
	return results
}

// consensusFork creates a new chain of execution results simulating a consensus fork.
// The method generates `n` new blocks descending from the provided parent header. Results are
// produced for each block, descending from the provided previous result ID.
func (s *ResultsForestSuite) consensusFork(
	n int,
	parentHeader *flow.Header,
	prevResultID flow.Identifier,
) []*flow.ExecutionResult {
	blockGen := s.g.Blocks()
	resultGen := s.g.ExecutionResults()

	blocks := blockGen.List(n,
		blockGen.WithParentHeader(parentHeader),
		// add a large gap to avoid conflicts with the main chain
		blockGen.WithView(parentHeader.View+100),
	)

	results := make([]*flow.ExecutionResult, len(blocks))
	for i, block := range blocks {
		s.allBlocks[block.ID()] = block

		results[i] = resultGen.Fixture(
			resultGen.WithBlock(block),
			resultGen.WithPreviousResultID(prevResultID),
		)
		prevResultID = results[i].ID()
	}
	return results
}

// balancedTree creates a complete balanced tree of execution results with degree `n`.
// there is one level for each block in the provided list of blocks, so the total depth is `len(blocks)`
// the root of the tree is the node represented by the parentHeader + prevResultID.
func (s *ResultsForestSuite) balancedTree(
	n int,
	prevResultID flow.Identifier,
	blocks []*flow.Block,
) []*flow.ExecutionResult {
	resultGen := s.g.ExecutionResults()
	block := blocks[0]

	children := make([]*flow.ExecutionResult, n)
	for i := range n {
		children[i] = resultGen.Fixture(
			resultGen.WithBlock(block),
			resultGen.WithPreviousResultID(prevResultID),
		)
	}

	if len(blocks) == 1 {
		return children
	}

	descendants := make([]*flow.ExecutionResult, 0)
	for _, child := range children {
		forkResults := s.balancedTree(n, child.ID(), blocks[1:])
		descendants = append(descendants, forkResults...)
	}
	return append(children, descendants...)
}

// chainWithSeals creates a chain of blocks, each with an execution results.
// Blocks contain seals for the result of block[n-2], and the first 2 blocks contain no seals.
func (s *ResultsForestSuite) chainWithSeals(
	n int,
	parentHeader *flow.Header,
	prevResultID flow.Identifier,
) ([]*flow.Block, []*flow.ExecutionResult) {
	blockGen := s.g.Blocks()
	payloadGen := s.g.Payloads()
	sealGen := s.g.Seals()
	resultGen := s.g.ExecutionResults()

	// generate block chain with 4 blocks, each with an execution result. blocks include a seal for
	// the result from 2 blocks prior.
	blocks := make([]*flow.Block, 0, n)
	results := make([]*flow.ExecutionResult, 0, n)

	for i := range n {
		blockOpts := []fixtures.BlockOption{
			blockGen.WithParentHeader(parentHeader),
		}
		if i < 2 {
			blockOpts = append(blockOpts, blockGen.WithPayload(
				payloadGen.Fixture(payloadGen.WithSeals()),
			))
		} else {
			blockOpts = append(blockOpts, blockGen.WithPayload(
				payloadGen.Fixture(payloadGen.WithSeals(
					sealGen.Fixture(
						sealGen.WithResultID(results[i-2].ID()),
						sealGen.WithBlockID(blocks[i-2].ID()),
					),
				)),
			))
		}

		block := blockGen.Fixture(blockOpts...)
		result := resultGen.Fixture(
			resultGen.WithBlock(block),
			resultGen.WithPreviousResultID(prevResultID),
		)
		parentHeader = block.ToHeader()
		prevResultID = result.ID()

		blocks = append(blocks, block)
		results = append(results, result)
		s.allBlocks[block.ID()] = block
	}

	return blocks, results
}

// addToForest adds the set of results to the forest with ResultForCertifiedBlock, and returns their pipelines
func (s *ResultsForestSuite) addToForest(forest *ingestion2.ResultsForest, results ...*flow.ExecutionResult) []*opsyncmock.Pipeline {
	receiptGen := s.g.ExecutionReceipts()

	pipelines := make([]*opsyncmock.Pipeline, len(results))
	for i, result := range results {
		pipelines[i] = s.createPipeline(result)
		pipelines[i].On("GetState").Return(optimistic_sync.StatePending).Maybe()

		receipt := receiptGen.Fixture(receiptGen.WithExecutionResult(*result))
		added, err := forest.AddReceipt(receipt, ingestion2.ResultForCertifiedBlock)
		s.Require().NoError(err)
		s.True(added)
	}
	return pipelines
}

package backend

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

// ExecutionResultsSuite tests backendExecutionResults directly, verifying correct delegation
// to storage and correct gRPC error code translation.
type ExecutionResultsSuite struct {
	suite.Suite

	g *fixtures.GeneratorSuite

	results  *storagemock.ExecutionResults
	seals    *storagemock.Seals
	receipts *storagemock.ExecutionReceipts

	backend backendExecutionResults
}

func TestExecutionResults(t *testing.T) {
	suite.Run(t, new(ExecutionResultsSuite))
}

func (s *ExecutionResultsSuite) SetupTest() {
	s.g = fixtures.NewGeneratorSuite()

	s.results = storagemock.NewExecutionResults(s.T())
	s.seals = storagemock.NewSeals(s.T())
	s.receipts = storagemock.NewExecutionReceipts(s.T())

	s.backend = backendExecutionResults{
		executionResults: s.results,
		seals:            s.seals,
		receipts:         s.receipts,
	}
}

// TestGetExecutionResultForBlockID tests the GetExecutionResultForBlockID method, which looks
// up the finalized seal for the block, then fetches the execution result referenced by the seal.
func (s *ExecutionResultsSuite) TestGetExecutionResultForBlockID() {
	ctx := context.Background()

	blockID := s.g.Identifiers().Fixture()
	result := s.g.ExecutionResults().Fixture(fixtures.ExecutionResult.WithBlockID(blockID))
	seal := s.g.Seals().Fixture(
		fixtures.Seal.WithBlockID(blockID),
		fixtures.Seal.WithResultID(result.ID()),
	)

	s.Run("not found - no seal for block", func() {
		unknownBlockID := s.g.Identifiers().Fixture()
		s.seals.On("FinalizedSealForBlock", unknownBlockID).
			Return(nil, storage.ErrNotFound).Once()

		_, err := s.backend.GetExecutionResultForBlockID(ctx, unknownBlockID)
		s.Require().Error(err)
		s.Assert().Equal(codes.NotFound, status.Code(err))
	})

	s.Run("not found - seal exists but result missing", func() {
		missingResultSeal := s.g.Seals().Fixture(fixtures.Seal.WithBlockID(blockID))
		s.seals.On("FinalizedSealForBlock", blockID).
			Return(missingResultSeal, nil).Once()
		s.results.On("ByID", missingResultSeal.ResultID).
			Return(nil, storage.ErrNotFound).Once()

		_, err := s.backend.GetExecutionResultForBlockID(ctx, blockID)
		s.Require().Error(err)
		s.Assert().Equal(codes.NotFound, status.Code(err))
	})

	// GetExecutionResultForBlockID uses rpc.ConvertStorageError, so unexpected storage
	// errors translate to codes.Internal without triggering irrecoverable.Throw.
	s.Run("exception - seals storage failure", func() {
		unknownBlockID := s.g.Identifiers().Fixture()
		s.seals.On("FinalizedSealForBlock", unknownBlockID).
			Return(nil, errors.New("seal db failure")).Once()

		_, err := s.backend.GetExecutionResultForBlockID(ctx, unknownBlockID)
		s.Require().Error(err)
		s.Assert().Equal(codes.Internal, status.Code(err))
	})

	s.Run("exception - results storage failure", func() {
		s.seals.On("FinalizedSealForBlock", blockID).
			Return(seal, nil).Once()
		s.results.On("ByID", seal.ResultID).
			Return(nil, errors.New("result db failure")).Once()

		_, err := s.backend.GetExecutionResultForBlockID(ctx, blockID)
		s.Require().Error(err)
		s.Assert().Equal(codes.Internal, status.Code(err))
	})

	s.Run("happy path", func() {
		s.seals.On("FinalizedSealForBlock", blockID).
			Return(seal, nil).Once()
		s.results.On("ByID", seal.ResultID).
			Return(result, nil).Once()

		actual, err := s.backend.GetExecutionResultForBlockID(ctx, blockID)
		s.Require().NoError(err)
		s.Assert().Equal(result, actual)
	})
}

// TestGetExecutionResultByID tests the GetExecutionResultByID method, which fetches
// an execution result directly by its ID.
func (s *ExecutionResultsSuite) TestGetExecutionResultByID() {
	ctx := context.Background()

	result := s.g.ExecutionResults().Fixture()

	s.Run("not found", func() {
		unknownID := s.g.Identifiers().Fixture()
		s.results.On("ByID", unknownID).
			Return(nil, storage.ErrNotFound).Once()

		_, err := s.backend.GetExecutionResultByID(ctx, unknownID)
		s.Require().Error(err)
		s.Assert().Equal(codes.NotFound, status.Code(err))
	})

	// GetExecutionResultByID uses rpc.ConvertStorageError, so unexpected storage
	// errors translate to codes.Internal without triggering irrecoverable.Throw.
	s.Run("exception - results storage failure", func() {
		unknownID := s.g.Identifiers().Fixture()
		s.results.On("ByID", unknownID).
			Return(nil, errors.New("result db failure")).Once()

		_, err := s.backend.GetExecutionResultByID(ctx, unknownID)
		s.Require().Error(err)
		s.Assert().Equal(codes.Internal, status.Code(err))
	})

	s.Run("happy path", func() {
		s.results.On("ByID", result.ID()).
			Return(result, nil).Once()

		actual, err := s.backend.GetExecutionResultByID(ctx, result.ID())
		s.Require().NoError(err)
		s.Assert().Equal(result, actual)
	})
}

// TestGetExecutionReceiptsByBlockID tests the GetExecutionReceiptsByBlockID method, which
// retrieves all execution receipts for a given block ID.
func (s *ExecutionResultsSuite) TestGetExecutionReceiptsByBlockID() {
	ctx := context.Background()

	blockID := s.g.Identifiers().Fixture()
	result1 := s.g.ExecutionResults().Fixture(fixtures.ExecutionResult.WithBlockID(blockID))
	result2 := s.g.ExecutionResults().Fixture(fixtures.ExecutionResult.WithBlockID(blockID))
	receipt1 := s.g.ExecutionReceipts().Fixture(fixtures.ExecutionReceipt.WithExecutionResult(*result1))
	receipt2 := s.g.ExecutionReceipts().Fixture(fixtures.ExecutionReceipt.WithExecutionResult(*result2))

	// GetExecutionReceiptsByBlockID calls irrecoverable.Throw for unexpected storage errors.
	s.Run("exception - receipts storage failure", func() {
		unknownBlockID := s.g.Identifiers().Fixture()
		storageErr := errors.New("receipts db failure")
		expectedThrow := fmt.Errorf("failed to get execution receipts by block ID: %w", storageErr)

		signalerCtx := irrecoverable.NewMockSignalerContextExpectError(s.T(), context.Background(), expectedThrow)
		ictx := irrecoverable.WithSignalerContext(context.Background(), signalerCtx)

		s.receipts.On("ByBlockID", unknownBlockID).Return(nil, storageErr).Once()

		_, err := s.backend.GetExecutionReceiptsByBlockID(ictx, unknownBlockID)
		s.Require().Error(err)
		s.Assert().True(errors.Is(err, storageErr))
	})

	s.Run("not found - empty list", func() {
		emptyBlockID := s.g.Identifiers().Fixture()
		s.receipts.On("ByBlockID", emptyBlockID).
			Return(flow.ExecutionReceiptList{}, nil).Once()

		_, err := s.backend.GetExecutionReceiptsByBlockID(ctx, emptyBlockID)
		s.Require().Error(err)
		s.Assert().Equal(codes.NotFound, status.Code(err))
	})

	s.Run("happy path - multiple receipts", func() {
		expected := flow.ExecutionReceiptList{receipt1, receipt2}
		s.receipts.On("ByBlockID", blockID).
			Return(expected, nil).Once()

		actual, err := s.backend.GetExecutionReceiptsByBlockID(ctx, blockID)
		s.Require().NoError(err)
		require.Equal(s.T(), []*flow.ExecutionReceipt(expected), actual)
	})
}

// TestGetExecutionReceiptsByResultID tests the GetExecutionReceiptsByResultID method, which
// resolves the block from the result, retrieves all receipts for that block, and filters
// to only those committing to the requested result ID.
func (s *ExecutionResultsSuite) TestGetExecutionReceiptsByResultID() {
	ctx := context.Background()

	blockID := s.g.Identifiers().Fixture()
	targetResult := s.g.ExecutionResults().Fixture(fixtures.ExecutionResult.WithBlockID(blockID))
	matchingReceipt1 := s.g.ExecutionReceipts().Fixture(fixtures.ExecutionReceipt.WithExecutionResult(*targetResult))
	matchingReceipt2 := s.g.ExecutionReceipts().Fixture(fixtures.ExecutionReceipt.WithExecutionResult(*targetResult))

	otherResult := s.g.ExecutionResults().Fixture(fixtures.ExecutionResult.WithBlockID(blockID))
	nonMatchingReceipt := s.g.ExecutionReceipts().Fixture(fixtures.ExecutionReceipt.WithExecutionResult(*otherResult))

	s.Run("not found - unknown result ID", func() {
		unknownID := s.g.Identifiers().Fixture()
		s.results.On("ByID", unknownID).
			Return(nil, storage.ErrNotFound).Once()

		_, err := s.backend.GetExecutionReceiptsByResultID(ctx, unknownID)
		s.Require().Error(err)
		s.Assert().Equal(codes.NotFound, status.Code(err))
	})

	// GetExecutionReceiptsByResultID calls irrecoverable.Throw for unexpected storage errors.
	s.Run("exception - results storage failure", func() {
		unknownID := s.g.Identifiers().Fixture()
		storageErr := errors.New("result db failure")
		expectedThrow := fmt.Errorf("failed to get execution result: %w", storageErr)

		signalerCtx := irrecoverable.NewMockSignalerContextExpectError(s.T(), context.Background(), expectedThrow)
		ictx := irrecoverable.WithSignalerContext(context.Background(), signalerCtx)

		s.results.On("ByID", unknownID).Return(nil, storageErr).Once()

		_, err := s.backend.GetExecutionReceiptsByResultID(ictx, unknownID)
		s.Require().Error(err)
		s.Assert().True(errors.Is(err, storageErr))
	})

	s.Run("exception - receipts storage failure", func() {
		storageErr := errors.New("receipts db failure")
		expectedThrow := fmt.Errorf("failed to get execution receipts by result ID: %w", storageErr)

		signalerCtx := irrecoverable.NewMockSignalerContextExpectError(s.T(), context.Background(), expectedThrow)
		ictx := irrecoverable.WithSignalerContext(context.Background(), signalerCtx)

		s.results.On("ByID", targetResult.ID()).Return(targetResult, nil).Once()
		s.receipts.On("ByBlockID", blockID).Return(nil, storageErr).Once()

		_, err := s.backend.GetExecutionReceiptsByResultID(ictx, targetResult.ID())
		s.Require().Error(err)
		s.Assert().True(errors.Is(err, storageErr))
	})

	s.Run("happy path - returns only matching receipts", func() {
		allReceipts := flow.ExecutionReceiptList{matchingReceipt1, nonMatchingReceipt, matchingReceipt2}
		s.results.On("ByID", targetResult.ID()).
			Return(targetResult, nil).Once()
		s.receipts.On("ByBlockID", blockID).
			Return(allReceipts, nil).Once()

		actual, err := s.backend.GetExecutionReceiptsByResultID(ctx, targetResult.ID())
		s.Require().NoError(err)
		s.Require().Len(actual, 2)
		s.Assert().ElementsMatch([]*flow.ExecutionReceipt{matchingReceipt1, matchingReceipt2}, actual)
	})

	s.Run("not found - no matching receipts for result", func() {
		s.results.On("ByID", targetResult.ID()).
			Return(targetResult, nil).Once()
		s.receipts.On("ByBlockID", blockID).
			Return(flow.ExecutionReceiptList{nonMatchingReceipt}, nil).Once()

		_, err := s.backend.GetExecutionReceiptsByResultID(ctx, targetResult.ID())
		s.Require().Error(err)
		s.Assert().Equal(codes.NotFound, status.Code(err))
	})
}

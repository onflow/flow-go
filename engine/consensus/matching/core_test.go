package matching

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMatchingCore(t *testing.T) {
	suite.Run(t, new(MatchingSuite))
}

type MatchingSuite struct {
	unittest.BaseChainSuite
	// misc SERVICE COMPONENTS which are injected into Sealing Core
	requester        *mockmodule.Requester
	receiptValidator *mockmodule.ReceiptValidator

	// MATCHING CORE
	core *Core
}

func (ms *MatchingSuite) SetupTest() {
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~ SETUP SUITE ~~~~~~~~~~~~~~~~~~~~~~~~~~ //
	ms.SetupChain()

	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()

	// ~~~~~~~~~~~~~~~~~~~~~~~ SETUP MATCHING CORE ~~~~~~~~~~~~~~~~~~~~~~~ //
	ms.requester = new(mockmodule.Requester)
	ms.receiptValidator = &mockmodule.ReceiptValidator{}

	config := Config{
		SealingThreshold:    10,
		MaxResultsToRequest: 200,
	}

	ms.core = NewCore(
		unittest.Logger(),
		tracer,
		metrics,
		metrics,
		ms.State,
		ms.HeadersDB,
		ms.ReceiptsDB,
		ms.ReceiptsPL,
		ms.PendingReceipts,
		ms.SealsPL,
		ms.receiptValidator,
		ms.requester,
		config,
	)
}

// Test that we reject receipts for unknown blocks without generating an error
func (ms *MatchingSuite) TestOnReceiptUnknownBlock() {
	// This receipt has a random block ID, so the sealing Core won't find it.
	receipt := unittest.ExecutionReceiptFixture()

	// onReceipt should reject the receipt without throwing an error
	_, err := ms.core.processReceipt(receipt)
	ms.Require().NoError(err, "should drop receipt for unknown block without error")

	ms.ReceiptsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// sealing Core should drop Result for known block that is already sealed
// without trying to store anything
func (ms *MatchingSuite) TestOnReceiptSealedResult() {
	originID := ms.ExeID
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.LatestSealedBlock))),
	)

	_, err := ms.core.processReceipt(receipt)
	ms.Require().NoError(err, "should ignore receipt for sealed result")

	ms.ReceiptsDB.AssertNumberOfCalls(ms.T(), "Store", 0)
}

// Test that we store different receipts for the same result
func (ms *MatchingSuite) TestOnReceiptPendingResult() {
	originID := ms.ExeID
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.UnfinalizedBlock))),
	)
	ms.receiptValidator.On("Validate", receipt).Return(nil)

	// Expect the receipt to be added to mempool and persistent storage
	ms.ReceiptsPL.On("AddReceipt", receipt, ms.UnfinalizedBlock.Header).Return(true, nil).Once()
	ms.ReceiptsDB.On("Store", receipt).Return(nil).Once()

	_, err := ms.core.processReceipt(receipt)
	ms.Require().NoError(err, "should handle different receipts for already pending result")
	ms.ReceiptsPL.AssertExpectations(ms.T())
	ms.ReceiptsDB.AssertExpectations(ms.T())
}

// TestOnReceipt_ReceiptInPersistentStorage verifies that Sealing Core adds
// a receipt to the mempool, even if it is already in persistent storage. This
// can happen after a crash, where the mempools got wiped
func (ms *MatchingSuite) TestOnReceipt_ReceiptInPersistentStorage() {
	originID := ms.ExeID
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.UnfinalizedBlock))),
	)
	ms.receiptValidator.On("Validate", receipt).Return(nil)

	// Persistent storage layer for Receipts has the receipt already stored
	ms.ReceiptsDB.On("Store", receipt).Return(storage.ErrAlreadyExists).Once()
	// The receipt should be added to the receipts mempool
	ms.ReceiptsPL.On("AddReceipt", receipt, ms.UnfinalizedBlock.Header).Return(true, nil).Once()

	_, err := ms.core.processReceipt(receipt)
	ms.Require().NoError(err, "should process receipts, even if it is already in storage")
	ms.ReceiptsPL.AssertExpectations(ms.T())
	ms.ReceiptsDB.AssertNumberOfCalls(ms.T(), "Store", 1)
}

// try to submit a receipt that should be valid
func (ms *MatchingSuite) TestOnReceiptValid() {
	originID := ms.ExeID
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.UnfinalizedBlock))),
	)

	ms.receiptValidator.On("Validate", receipt).Return(nil).Once()

	// Expect the receipt to be added to mempool and persistent storage
	ms.ReceiptsPL.On("AddReceipt", receipt, ms.UnfinalizedBlock.Header).Return(true, nil).Once()
	ms.ReceiptsDB.On("Store", receipt).Return(nil).Once()

	// onReceipt should run to completion without throwing an error
	_, err := ms.core.processReceipt(receipt)
	ms.Require().NoError(err, "should add receipt and result to mempools if valid")

	ms.receiptValidator.AssertExpectations(ms.T())
	ms.ReceiptsPL.AssertExpectations(ms.T())
	ms.ReceiptsDB.AssertExpectations(ms.T())
}

// TestOnReceiptInvalid tests handing of receipts that the ReceiptValidator detects as violating the protocol
func (ms *MatchingSuite) TestOnReceiptInvalid() {
	// we use the same Receipt as in TestOnReceiptValid to ensure that the sealing Core is not
	// rejecting the receipt for any other reason
	originID := ms.ExeID
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.UnfinalizedBlock))),
	)

	// check that _expected_ failure case of invalid receipt is handled without error
	ms.receiptValidator.On("Validate", receipt).Return(engine.NewInvalidInputError("")).Once()
	wasAdded, err := ms.core.processReceipt(receipt)
	ms.Require().NoError(err, "invalid receipt should be dropped but not error")
	ms.Require().False(wasAdded, "invalid receipt should not be added")
	ms.receiptValidator.AssertExpectations(ms.T())
	ms.ReceiptsDB.AssertNumberOfCalls(ms.T(), "Store", 0)
}

// TestOnReceiptValidatorExceptions tests matching.Core escalates unexpected errors and exceptions.
// We expect that such errors are *not* interpreted as the receipt being invalid.
func (ms *MatchingSuite) TestOnReceiptValidatorExceptions() {
	// we use the same Receipt as in TestOnReceiptValid to ensure that the sealing Core is not rejecting the receipt for any other reason
	originID := ms.ExeID
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.UnfinalizedBlock))),
	)

	// Check that _unexpected_ failure causes the error to be escalated and is *not* interpreted as an invalid receipt.
	ms.receiptValidator.On("Validate", receipt).Return(fmt.Errorf("")).Once()
	_, err := ms.core.processReceipt(receipt)
	ms.Require().Error(err, "unexpected errors should be escalated")
	ms.Require().False(engine.IsInvalidInputError(err), "exceptions should not be misinterpreted as an invalid receipt")

	// Check that an `UnknownBlockError` causes the error to be escalated and is *not* interpreted as an invalid receipt.
	// Reasoning: For attack resilience, we should discard outdated receipts based on the height of the executed block, _before_ we
	// run the expensive receipt validation. Therefore, matching.Core should retrieve the executed block before calling into the
	// ReceiptValidator. Hence, if matching.Core finds the executed block, but `ReceiptValidator.Validate(..)` errors saying that
	// the executed block is unknown, our state is corrupted or we have a severe internal bug.
	ms.receiptValidator.On("Validate", receipt).Return(module.NewUnknownBlockError("")).Once()
	_, err = ms.core.processReceipt(receipt)
	ms.Require().Error(err, "unexpected errors should be escalated")
	ms.Require().False(engine.IsInvalidInputError(err), "exceptions should not be misinterpreted as an invalid receipt")

	ms.receiptValidator.AssertExpectations(ms.T())
	ms.ReceiptsDB.AssertNumberOfCalls(ms.T(), "Store", 0)
}

// TestOnUnverifiableReceipt tests handling of receipts that are unverifiable
// (e.g. if the parent result is unknown)
func (ms *MatchingSuite) TestOnUnverifiableReceipt() {
	// we use the same Receipt as in TestOnReceiptValid to ensure that the matching Core is not
	// rejecting the receipt for any other reason
	originID := ms.ExeID
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.UnfinalizedBlock))),
	)

	ms.PendingReceipts.On("Add", receipt).Return(false).Once()

	// check that _expected_ failure case of invalid receipt is handled without error
	ms.receiptValidator.On("Validate", receipt).Return(module.NewUnknownResultError("missing parent result")).Once()
	wasAdded, err := ms.core.processReceipt(receipt)
	ms.Require().NoError(err, "unverifiable receipt should be cached but not error")
	ms.Require().False(wasAdded, "unverifiable receipt should be cached but not added to the node's validated information")

	ms.receiptValidator.AssertExpectations(ms.T())
	ms.ReceiptsDB.AssertNumberOfCalls(ms.T(), "Store", 0)
	ms.PendingReceipts.AssertExpectations(ms.T())
}

// TestRequestPendingReceipts tests sealing.Core.requestPendingReceipts():
//   - generate n=100 consecutive blocks, where the first one is sealed and the last one is final
func (ms *MatchingSuite) TestRequestPendingReceipts() {
	// create blocks
	n := 100
	orderedBlocks := make([]flow.Block, 0, n)
	parentBlock := ms.UnfinalizedBlock
	for i := 0; i < n; i++ {
		block := unittest.BlockWithParentFixture(parentBlock.Header)
		ms.Extend(block)
		orderedBlocks = append(orderedBlocks, *block)
		parentBlock = *block
	}

	// progress latest sealed and latest finalized:
	ms.LatestSealedBlock = orderedBlocks[0]
	ms.LatestFinalizedBlock = &orderedBlocks[n-1]

	// Expecting all blocks to be requested: from sealed height + 1 up to (incl.) latest finalized
	for i := 1; i < n; i++ {
		id := orderedBlocks[i].ID()
		ms.requester.On("Query", id, mock.Anything).Return().Once()
	}
	ms.SealsPL.On("All").Return([]*flow.IncorporatedResultSeal{}).Maybe()

	// we have no receipts
	ms.ReceiptsDB.On("ByBlockID", mock.Anything).Return(nil, nil)

	_, _, err := ms.core.requestPendingReceipts()
	ms.Require().NoError(err, "should request results for pending blocks")
	ms.requester.AssertExpectations(ms.T()) // asserts that requester.Query(<blockID>, filter.Any) was called
}

// TestRequestSecondPendingReceipt verifies that a second receipt is re-requested
// Situation A:
//   - we have _once_ receipt for an unsealed finalized block in storage
//   - Expected: Method Core.requestPendingReceipts() should re-request a second receipt
//
// Situation B:
//   - we have _two_ receipts for an unsealed finalized block storage
//   - Expected: Method Core.requestPendingReceipts() should _not_ request another receipt
//
// TODO: this test is temporarily requires as long as sealing.Core requires _two_ receipts from different ENs to seal
func (ms *MatchingSuite) TestRequestSecondPendingReceipt() {

	ms.core.config.SealingThreshold = 0 // request receipts for all unsealed finalized blocks

	result := unittest.ExecutionResultFixture(unittest.WithBlock(ms.LatestFinalizedBlock))

	// make receipts:
	receipt1 := unittest.ExecutionReceiptFixture(unittest.WithResult(result))
	receipt2 := unittest.ExecutionReceiptFixture(unittest.WithResult(result))

	// receipts from storage are potentially added to receipts mempool and incorporated results mempool
	ms.ReceiptsPL.On("AddReceipt", receipt1, ms.LatestFinalizedBlock.Header).Return(false, nil).Maybe()
	ms.ReceiptsPL.On("AddReceipt", receipt2, ms.LatestFinalizedBlock.Header).Return(false, nil).Maybe()

	// Situation A: we have _once_ receipt for an unsealed finalized block in storage
	ms.ReceiptsDB.On("ByBlockID", ms.LatestFinalizedBlock.ID()).Return(flow.ExecutionReceiptList{receipt1}, nil).Once()
	ms.requester.On("Query", ms.LatestFinalizedBlock.ID(), mock.Anything).Return().Once() // Core should trigger requester to re-request a second receipt
	_, _, err := ms.core.requestPendingReceipts()
	ms.Require().NoError(err, "should request results for pending blocks")
	ms.requester.AssertExpectations(ms.T()) // asserts that requester.Query(<blockID>, filter.Any) was called

	// Situation B: we have _two_ receipts for an unsealed finalized block storage
	ms.ReceiptsDB.On("ByBlockID", ms.LatestFinalizedBlock.ID()).Return(flow.ExecutionReceiptList{receipt1, receipt2}, nil).Once()
	_, _, err = ms.core.requestPendingReceipts()
	ms.Require().NoError(err, "should request results for pending blocks")
	ms.requester.AssertExpectations(ms.T()) // asserts that requester.Query(<blockID>, filter.Any) was called
}

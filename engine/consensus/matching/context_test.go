package matching

import (
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMatchingEngineContext(t *testing.T) {
	suite.Run(t, new(EngineContextSuite))
}

type EngineContextSuite struct {
	unittest.BaseChainSuite
	// misc SERVICE COMPONENTS which are injected into Matching Engine
	requester        *mockmodule.Requester
	receiptValidator *mockmodule.ReceiptValidator

	// Context
	context *EngineContext
}

func (ms *EngineContextSuite) SetupTest() {
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~ SETUP SUITE ~~~~~~~~~~~~~~~~~~~~~~~~~~ //
	ms.SetupChain()

	unit := engine.NewUnit()
	log := zerolog.New(os.Stderr)
	metrics := metrics.NewNoopCollector()

	// ~~~~~~~~~~~~~~~~~~~~~~~ SETUP MATCHING ENGINE ~~~~~~~~~~~~~~~~~~~~~~~ //
	ms.requester = new(mockmodule.Requester)
	ms.receiptValidator = &mockmodule.ReceiptValidator{}

	approvalsProvider := &concurrentQueueProvider{}
	approvalResponseProvider := &concurrentQueueProvider{}
	receiptsProvider := &concurrentQueueProvider{}

	ms.context = &EngineContext{
		log: log,
		engine: &Engine{
			unit:                                 unit,
			log:                                  log,
			engineMetrics:                        metrics,
			mempool:                              metrics,
			metrics:                              metrics,
			state:                                ms.State,
			receiptRequester:                     ms.requester,
			receiptsDB:                           ms.ReceiptsDB,
			headersDB:                            ms.HeadersDB,
			indexDB:                              ms.IndexDB,
			incorporatedResults:                  ms.ResultsPL,
			receipts:                             ms.ReceiptsPL,
			approvals:                            ms.ApprovalsPL,
			seals:                                ms.SealsPL,
			isCheckingSealing:                    atomic.NewBool(false),
			sealingThreshold:                     10,
			maxResultsToRequest:                  200,
			assigner:                             ms.Assigner,
			receiptValidator:                     ms.receiptValidator,
			requestTracker:                       NewRequestTracker(1, 3),
			approvalRequestsThreshold:            10,
			requiredApprovalsForSealConstruction: DefaultRequiredApprovalsForSealConstruction,
			emergencySealingActive:               false,
			resultApprovalsQueue:                 approvalsProvider,
			approvalResponsesQueue:               approvalResponseProvider,
			receiptsQueue:                        receiptsProvider,
		},
		approvalsProvider:        approvalsProvider,
		approvalResponseProvider: approvalResponseProvider,
		receiptsProvider:         receiptsProvider,
	}
}

// try to submit a receipt event that should be valid
func (ms *EngineContextSuite) TestProcessValidReceipt() {
	originID := ms.ExeID
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&ms.UnfinalizedBlock))),
	)

	ms.receiptValidator.On("Validate", []*flow.ExecutionReceipt{receipt}).Return(nil).Once()

	// we expect that receipt is added to mempool
	ms.ReceiptsPL.On("AddReceipt", receipt, ms.UnfinalizedBlock.Header).Return(true, nil).Once()

	// setup the results mempool to check if we attempted to add the incorporated result
	ms.ResultsPL.
		On("Add", incorporatedResult(receipt.ExecutionResult.BlockID, &receipt.ExecutionResult)).
		Return(true, nil).Once()

	<-ms.context.Ready()

	err := ms.context.Process(originID, receipt)
	ms.Require().NoError(err, "should add receipt and result to mempool if valid")
	ms.Require().Equal(1, ms.context.receiptsProvider.q.Len())

	// matching engine has at least 100ms ticks for processing events
	time.Sleep(1 * time.Second)

	ms.receiptValidator.AssertExpectations(ms.T())
	ms.ReceiptsPL.AssertExpectations(ms.T())
	ms.ResultsPL.AssertExpectations(ms.T())
}

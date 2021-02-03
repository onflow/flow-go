package matching

import (
	"github.com/onflow/flow-go/module/trace"
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
	// misc SERVICE COMPONENTS which are injected into Matching Core
	requester        *mockmodule.Requester
	receiptValidator *mockmodule.ReceiptValidator

	// Context
	context *Engine
}

//func (ms *EngineContextSuite) TearDownTest() {
//<-ms.context.Done()
//}

func (ms *EngineContextSuite) SetupTest() {
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~ SETUP SUITE ~~~~~~~~~~~~~~~~~~~~~~~~~~ //
	ms.SetupChain()

	log := zerolog.New(os.Stderr)
	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()

	// ~~~~~~~~~~~~~~~~~~~~~~~ SETUP MATCHING ENGINE ~~~~~~~~~~~~~~~~~~~~~~~ //
	ms.requester = new(mockmodule.Requester)
	ms.receiptValidator = &mockmodule.ReceiptValidator{}

	approvalsProvider := make(chan *Event)
	approvalResponseProvider := make(chan *Event)
	receiptsProvider := make(chan *Event)

	ms.context = &Engine{
		log:  log,
		unit: engine.NewUnit(),
		engine: &Core{
			tracer:                               tracer,
			unit:                                 engine.NewUnit(),
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
			approvalEventProvider:                approvalsProvider,
			approvalResponseEventProvider:        approvalResponseProvider,
			receiptEventProvider:                 receiptsProvider,
		},
		approvalSink:         approvalsProvider,
		approvalResponseSink: approvalResponseProvider,
		receiptSink:          receiptsProvider,
		pendingEventSink:     make(chan *Event),
		engineMetrics:        metrics,
	}

	<-ms.context.Ready()
}

// TestProcessValidReceipt tests if valid receipt gets recorded into mempool when send through `Engine`.
// Tests the whole processing pipeline.
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

	err := ms.context.Process(originID, receipt)
	ms.Require().NoError(err, "should add receipt and result to mempool if valid")

	// matching engine has at least 100ms ticks for processing events
	time.Sleep(1 * time.Second)

	ms.receiptValidator.AssertExpectations(ms.T())
	ms.ReceiptsPL.AssertExpectations(ms.T())
	ms.ResultsPL.AssertExpectations(ms.T())
}

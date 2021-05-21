// (c) 2021 Dapper Labs - ALL RIGHTS RESERVED

package sealing

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	mockconsensus "github.com/onflow/flow-go/engine/consensus/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSealingEngineContext(t *testing.T) {
	suite.Run(t, new(SealingEngineSuite))
}

type SealingEngineSuite struct {
	suite.Suite

	core *mockconsensus.SealingCore

	// Sealing Engine
	engine *Engine
}

func (s *SealingEngineSuite) SetupTest() {
	log := zerolog.New(os.Stderr)
	metrics := metrics.NewNoopCollector()
	me := &mockmodule.Local{}
	s.core = &mockconsensus.SealingCore{}

	s.engine = &Engine{
		log:                                  log,
		unit:                                 engine.NewUnit(),
		core:                                 s.core,
		me:                                   me,
		approvalSink:                         make(chan *Event),
		requestedApprovalSink:                make(chan *Event),
		receiptSink:                          make(chan *Event),
		pendingEventSink:                     make(chan *Event),
		engineMetrics:                        metrics,
		cacheMetrics:                         metrics,
		workerPool:                           workerpool.New(8),
		requiredApprovalsForSealConstruction: RequiredApprovalsForSealConstructionTestingValue,
	}

	s.engine.pendingReceipts, _ = fifoqueue.NewFifoQueue()
	s.engine.pendingApprovals, _ = fifoqueue.NewFifoQueue()
	s.engine.pendingRequestedApprovals, _ = fifoqueue.NewFifoQueue()

	<-s.engine.Ready()
}

// TestOnFinalizedBlock tests if finalized block gets processed when send through `Engine`.
// Tests the whole processing pipeline.
func (s *SealingEngineSuite) TestOnFinalizedBlock() {
	finalizedBlockID := unittest.IdentifierFixture()
	// setup payload fixture
	payloads := &mockstorage.Payloads{}
	payload := unittest.PayloadFixture()
	payloads.On("ByBlockID", finalizedBlockID).Return(&payload, nil).Once()
	s.engine.payloads = payloads

	s.core.On("ProcessFinalizedBlock", finalizedBlockID).Return(nil).Once()
	s.engine.OnFinalizedBlock(finalizedBlockID)

	// matching engine has at least 100ms ticks for processing events
	time.Sleep(1 * time.Second)

	s.core.AssertExpectations(s.T())
}

// TestOnBlockIncorporated tests if incorporated block gets processed when send through `Engine`.
// Tests the whole processing pipeline.
func (s *SealingEngineSuite) TestOnBlockIncorporated() {
	parentBlock := unittest.BlockHeaderFixture()
	incorporatedBlock := unittest.BlockHeaderWithParentFixture(&parentBlock)
	incorporatedBlockID := incorporatedBlock.ID()
	// setup payload fixture
	payloads := &mockstorage.Payloads{}
	payload := unittest.PayloadFixture()
	unittest.WithAllTheFixins(&payload)
	payloads.On("ByBlockID", parentBlock.ID()).Return(&payload, nil).Once()
	s.engine.payloads = payloads

	// setup headers storage
	headers := &mockstorage.Headers{}
	headers.On("ByBlockID", incorporatedBlockID).Return(&incorporatedBlock, nil).Once()
	s.engine.headers = headers

	for _, result := range payload.Results {
		IR := flow.NewIncorporatedResult(result.BlockID, result)
		s.core.On("ProcessIncorporatedResult", IR).Return(nil).Once()
	}

	s.engine.OnBlockIncorporated(incorporatedBlockID)

	// matching engine has at least 100ms ticks for processing events
	time.Sleep(1 * time.Second)

	s.core.AssertExpectations(s.T())
}

// TestProcessValidReceipt tests if valid receipt gets recorded into mempool when send through `Engine`.
// Tests the whole processing pipeline.
func (s *SealingEngineSuite) TestProcessValidReceipt() {
	block := unittest.BlockFixture()
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&block))),
	)

	originID := unittest.IdentifierFixture()

	IR := flow.NewIncorporatedResult(receipt.ExecutionResult.BlockID, &receipt.ExecutionResult)
	s.core.On("ProcessIncorporatedResult", IR).Return(nil).Once()

	err := s.engine.Process(originID, receipt)
	s.Require().NoError(err, "should add receipt and result to mempool if valid")

	// sealing engine has at least 100ms ticks for processing events
	time.Sleep(1 * time.Second)

	s.core.AssertExpectations(s.T())
}

// TestMultipleProcessingItems tests that the engine queues multiple receipts and approvals
// and eventually feeds them into sealing.Core for processing
func (s *SealingEngineSuite) TestMultipleProcessingItems() {
	originID := unittest.IdentifierFixture()
	block := unittest.BlockFixture()

	receipts := make([]*flow.ExecutionReceipt, 20)
	for i := range receipts {
		receipt := unittest.ExecutionReceiptFixture(
			unittest.WithExecutorID(originID),
			unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&block))),
		)
		receipts[i] = receipt
		IR := flow.NewIncorporatedResult(receipt.ExecutionResult.BlockID, &receipt.ExecutionResult)
		s.core.On("ProcessIncorporatedResult", IR).Return(nil).Once()
	}

	numApprovalsPerReceipt := 1
	approvals := make([]*flow.ResultApproval, 0, len(receipts)*numApprovalsPerReceipt)
	approverID := unittest.IdentifierFixture()
	for _, receipt := range receipts {
		for j := 0; j < numApprovalsPerReceipt; j++ {
			approval := unittest.ResultApprovalFixture(unittest.WithExecutionResultID(receipt.ID()),
				unittest.WithApproverID(approverID))
			approvals = append(approvals, approval)
			s.core.On("ProcessApproval", approval).Return(nil).Once()
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, receipt := range receipts {
			err := s.engine.Process(originID, receipt)
			s.Require().NoError(err, "should add receipt and result to mempool if valid")
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, approval := range approvals {
			err := s.engine.Process(approverID, approval)
			s.Require().NoError(err, "should process approval")
		}
	}()

	wg.Wait()

	// sealing engine has at least 100ms ticks for processing events
	time.Sleep(1 * time.Second)

	s.core.AssertExpectations(s.T())
}

// try to submit an approval where the message origin is inconsistent with the message creator
func (s *SealingEngineSuite) TestApprovalInvalidOrigin() {
	// approval from valid origin (i.e. a verification node) but with random ApproverID
	originID := unittest.IdentifierFixture()
	approval := unittest.ResultApprovalFixture() // with random ApproverID

	err := s.engine.Process(originID, approval)
	s.Require().NoError(err, "approval from unknown verifier should be dropped but not error")

	// sealing engine has at least 100ms ticks for processing events
	time.Sleep(1 * time.Second)

	// In both cases, we expect the approval to be rejected without hitting the mempools
	s.core.AssertNumberOfCalls(s.T(), "ProcessApproval", 0)
}

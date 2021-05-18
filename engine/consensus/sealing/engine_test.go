// (c) 2021 Dapper Labs - ALL RIGHTS RESERVED

package sealing

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	mockconsensus "github.com/onflow/flow-go/engine/consensus/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestApprovalsEngineContext(t *testing.T) {
	suite.Run(t, new(ApprovalsEngineSuite))
}

type ApprovalsEngineSuite struct {
	suite.Suite

	core *mockconsensus.ResultApprovalProcessor

	// Sealing Engine
	engine *Engine
}

func (ms *ApprovalsEngineSuite) SetupTest() {
	log := zerolog.New(os.Stderr)
	metrics := metrics.NewNoopCollector()
	me := &mockmodule.Local{}
	net := &mockmodule.Network{}
	ms.core = &mockconsensus.ResultApprovalProcessor{}

	receiptsCon := &mocknetwork.Conduit{}
	approvalsCon := &mocknetwork.Conduit{}
	requestApprovalsCon := &mocknetwork.Conduit{}

	net.On("Register", engine.ReceiveReceipts, testifymock.Anything).
		Return(receiptsCon, nil).
		Once()
	net.On("Register", engine.ReceiveApprovals, testifymock.Anything).
		Return(approvalsCon, nil).
		Once()
	net.On("Register", engine.RequestApprovalsByChunk, testifymock.Anything).
		Return(requestApprovalsCon, nil).
		Once()

	var err error
	ms.engine, err = NewEngine(log, metrics, ms.core, metrics, net, me, 1)
	require.NoError(ms.T(), err)
	<-ms.engine.Ready()
}

// TestProcessValidReceipt tests if valid receipt gets recorded into mempool when send through `Engine`.
// Tests the whole processing pipeline.
func (ms *ApprovalsEngineSuite) TestProcessValidReceipt() {
	block := unittest.BlockFixture()
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&block))),
	)

	originID := unittest.IdentifierFixture()

	IR := flow.NewIncorporatedResult(receipt.ExecutionResult.BlockID, &receipt.ExecutionResult)
	ms.core.On("ProcessIncorporatedResult", IR).Return(nil).Once()

	err := ms.engine.Process(originID, receipt)
	ms.Require().NoError(err, "should add receipt and result to mempool if valid")

	// sealing engine has at least 100ms ticks for processing events
	time.Sleep(1 * time.Second)

	ms.core.AssertExpectations(ms.T())
}

// TestMultipleProcessingItems tests that the engine queues multiple receipts and approvals
// and eventually feeds them into sealing.Core for processing
func (ms *ApprovalsEngineSuite) TestMultipleProcessingItems() {
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
		ms.core.On("ProcessIncorporatedResult", IR).Return(nil).Once()
	}

	numApprovalsPerReceipt := 1
	approvals := make([]*flow.ResultApproval, 0, len(receipts)*numApprovalsPerReceipt)
	approverID := unittest.IdentifierFixture()
	for _, receipt := range receipts {
		for j := 0; j < numApprovalsPerReceipt; j++ {
			approval := unittest.ResultApprovalFixture(unittest.WithExecutionResultID(receipt.ID()),
				unittest.WithApproverID(approverID))
			approvals = append(approvals, approval)
			ms.core.On("ProcessApproval", approval).Return(nil).Once()
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, receipt := range receipts {
			err := ms.engine.Process(originID, receipt)
			ms.Require().NoError(err, "should add receipt and result to mempool if valid")
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, approval := range approvals {
			err := ms.engine.Process(approverID, approval)
			ms.Require().NoError(err, "should process approval")
		}
	}()

	wg.Wait()

	// sealing engine has at least 100ms ticks for processing events
	time.Sleep(1 * time.Second)

	ms.core.AssertExpectations(ms.T())
}

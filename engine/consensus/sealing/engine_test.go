package sealing

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	mockconsensus "github.com/onflow/flow-go/engine/consensus/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSealingEngineContext(t *testing.T) {
	suite.Run(t, new(SealingEngineSuite))
}

type SealingEngineSuite struct {
	suite.Suite

	core    *mockconsensus.SealingCore
	state   *mockprotocol.State
	index   *mockstorage.Index
	results *mockstorage.ExecutionResults
	myID    flow.Identifier

	// Sealing Engine
	engine *Engine
	cancel context.CancelFunc
}

func (s *SealingEngineSuite) SetupTest() {
	metrics := metrics.NewNoopCollector()
	s.core = &mockconsensus.SealingCore{}
	s.state = &mockprotocol.State{}
	s.index = &mockstorage.Index{}
	s.results = &mockstorage.ExecutionResults{}
	s.myID = unittest.IdentifierFixture()
	me := &mockmodule.Local{}
	// set up local module mock
	me.On("NodeID").Return(
		func() flow.Identifier {
			return s.myID
		},
	)

	rootHeader, err := unittest.RootSnapshotFixture(unittest.IdentityListFixture(5, unittest.WithAllRoles())).Head()
	require.NoError(s.T(), err)

	s.engine = &Engine{
		log:           unittest.Logger(),
		workerPool:    workerpool.New(defaultAssignmentCollectorsWorkerPoolCapacity),
		core:          s.core,
		me:            me,
		engineMetrics: metrics,
		cacheMetrics:  metrics,
		rootHeader:    rootHeader,
		index:         s.index,
		results:       s.results,
		state:         s.state,
	}

	// setup inbound queues for trusted inputs and message handler for untrusted inputs
	err = s.engine.setupTrustedInboundQueues()
	require.NoError(s.T(), err)
	err = s.engine.setupMessageHandler(unittest.NewSealingConfigs(RequiredApprovalsForSealConstructionTestingValue))
	require.NoError(s.T(), err)

	// setup ComponentManager and start the engine
	s.engine.Component = s.engine.buildComponentManager()
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(s.T(), context.Background())
	s.cancel = cancel
	s.engine.Start(ctx)
	unittest.AssertClosesBefore(s.T(), s.engine.Ready(), 10*time.Millisecond)
}

func (s *SealingEngineSuite) TearDownTest() {
	if s.cancel != nil {
		s.cancel()
		unittest.AssertClosesBefore(s.T(), s.engine.Done(), 10*time.Millisecond)
	}
}

// TestOnFinalizedBlock tests if finalized block gets processed when sent through [Engine].
// Tests the whole processing pipeline.
func (s *SealingEngineSuite) TestOnFinalizedBlock() {

	finalizedBlock := unittest.BlockHeaderFixture()
	finalizedBlockID := finalizedBlock.ID()

	s.state.On("Final").Return(unittest.StateSnapshotForKnownBlock(finalizedBlock, nil))
	s.core.On("ProcessFinalizedBlock", finalizedBlockID).Return(nil).Once()
	s.engine.OnFinalizedBlock(model.BlockFromFlow(finalizedBlock))

	// matching engine has at least 100ms ticks for processing events
	time.Sleep(1 * time.Second)

	s.core.AssertExpectations(s.T())
}

// TestOnBlockIncorporated tests if incorporated block gets processed when sent through [Engine].
// Tests the whole processing pipeline.
func (s *SealingEngineSuite) TestOnBlockIncorporated() {
	parentBlock := unittest.BlockHeaderFixture()
	incorporatedBlock := unittest.BlockHeaderWithParentFixture(parentBlock)
	incorporatedBlockID := incorporatedBlock.ID()
	// setup payload fixture
	payload := unittest.PayloadFixture(unittest.WithAllTheFixins)
	index := &flow.Index{}

	for _, result := range payload.Results {
		index.ResultIDs = append(index.ReceiptIDs, result.ID())
		s.results.On("ByID", result.ID()).Return(result, nil).Once()

		IR, err := flow.NewIncorporatedResult(flow.UntrustedIncorporatedResult{
			IncorporatedBlockID: parentBlock.ID(),
			Result:              result,
		})
		require.NoError(s.T(), err)
		s.core.On("ProcessIncorporatedResult", IR).Return(nil).Once()
	}
	s.index.On("ByBlockID", parentBlock.ID()).Return(index, nil)

	// setup headers storage
	headers := &mockstorage.Headers{}
	headers.On("ByBlockID", incorporatedBlockID).Return(incorporatedBlock, nil).Once()
	s.engine.headers = headers

	s.engine.OnBlockIncorporated(model.BlockFromFlow(incorporatedBlock))

	// matching engine has at least 100ms ticks for processing events
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
	}

	numApprovalsPerReceipt := 1
	approvals := make([]*flow.ResultApproval, 0, len(receipts)*numApprovalsPerReceipt)
	responseApprovals := make([]*messages.ApprovalResponse, 0)
	approverID := unittest.IdentifierFixture()
	for _, receipt := range receipts {
		for j := 0; j < numApprovalsPerReceipt; j++ {
			approval := unittest.ResultApprovalFixture(unittest.WithExecutionResultID(receipt.ID()),
				unittest.WithApproverID(approverID))
			responseApproval := &messages.ApprovalResponse{
				Approval: *approval,
			}
			responseApprovals = append(responseApprovals, responseApproval)
			approvals = append(approvals, approval)
			s.core.On("ProcessApproval", approval).Return(nil).Twice()
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, approval := range approvals {
			err := s.engine.Process(channels.ReceiveApprovals, approverID, approval)
			s.Require().NoError(err, "should process approval")
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, approval := range responseApprovals {
			err := s.engine.Process(channels.ReceiveApprovals, approverID, approval)
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

	err := s.engine.Process(channels.ReceiveApprovals, originID, approval)
	s.Require().NoError(err, "approval from unknown verifier should be dropped but not error")

	// sealing engine has at least 100ms ticks for processing events
	time.Sleep(1 * time.Second)

	// In both cases, we expect the approval to be rejected without hitting the mempools
	s.core.AssertNumberOfCalls(s.T(), "ProcessApproval", 0)
}

// TestProcessUnsupportedMessageType tests that Process correctly handles a case where invalid message type
// was submitted from network layer.
func (s *SealingEngineSuite) TestProcessUnsupportedMessageType() {
	invalidEvent := uint64(42)
	err := s.engine.Process("ch", unittest.IdentifierFixture(), invalidEvent)
	// shouldn't result in error since byzantine inputs are expected
	require.NoError(s.T(), err)
}

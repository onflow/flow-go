package matching

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	mockconsensus "github.com/onflow/flow-go/engine/consensus/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	mocknetwork "github.com/onflow/flow-go/network/mock"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMatchingEngineContext(t *testing.T) {
	suite.Run(t, new(MatchingEngineSuite))
}

type MatchingEngineSuite struct {
	suite.Suite

	index    *mockstorage.Index
	receipts *mockstorage.ExecutionReceipts
	core     *mockconsensus.MatchingCore
	state    *mockprotocol.State

	// Matching Engine
	engine *Engine
	cancel context.CancelFunc
}

func (s *MatchingEngineSuite) SetupTest() {
	metrics := metrics.NewNoopCollector()
	me := &mockmodule.Local{}
	net := &mocknetwork.EngineRegistry{}
	s.core = &mockconsensus.MatchingCore{}
	s.index = &mockstorage.Index{}
	s.receipts = &mockstorage.ExecutionReceipts{}
	s.state = &mockprotocol.State{}

	ourNodeID := unittest.IdentifierFixture()
	me.On("NodeID").Return(ourNodeID)

	con := &mocknetwork.Conduit{}
	net.On("Register", mock.Anything, mock.Anything).Return(con, nil).Once()

	var err error
	s.engine, err = NewEngine(unittest.Logger(), net, me, metrics, metrics, s.state, s.receipts, s.index, s.core)
	require.NoError(s.T(), err)

	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(s.T(), context.Background())
	s.cancel = cancel
	s.engine.Start(ctx)
	unittest.AssertClosesBefore(s.T(), s.engine.Ready(), 10*time.Millisecond)
}

func (s *MatchingEngineSuite) TearDownTest() {
	if s.cancel != nil {
		s.cancel()
		unittest.AssertClosesBefore(s.T(), s.engine.Done(), 10*time.Millisecond)
	}
}

// TestOnFinalizedBlock tests if finalized block gets processed when send through `Engine`.
// Tests the whole processing pipeline.
func (s *MatchingEngineSuite) TestOnFinalizedBlock() {

	finalizedBlock := unittest.BlockHeaderFixture()
	s.state.On("Final").Return(unittest.StateSnapshotForKnownBlock(finalizedBlock, nil))
	s.core.On("OnBlockFinalization").Return(nil).Once()
	s.engine.OnFinalizedBlock(model.BlockFromFlow(finalizedBlock))

	// matching engine has at least 100ms ticks for processing events
	time.Sleep(1 * time.Second)

	s.core.AssertExpectations(s.T())
}

// TestOnBlockIncorporated tests if incorporated block gets processed when send through `Engine`.
// Tests the whole processing pipeline.
func (s *MatchingEngineSuite) TestOnBlockIncorporated() {

	incorporatedBlock := unittest.BlockHeaderFixture()
	incorporatedBlockID := incorporatedBlock.Hash()

	payload := unittest.PayloadFixture(unittest.WithAllTheFixins)
	index := &flow.Index{}
	resultsByID := payload.Results.Lookup()
	for _, receipt := range payload.Receipts {
		index.ReceiptIDs = append(index.ReceiptIDs, receipt.Hash())
		fullReceipt, err := flow.ExecutionReceiptFromStub(*receipt, *resultsByID[receipt.ResultID])
		s.Require().NoError(err)
		s.receipts.On("ByID", receipt.Hash()).Return(fullReceipt, nil).Once()
		s.core.On("ProcessReceipt", fullReceipt).Return(nil).Once()
	}
	s.index.On("ByBlockID", incorporatedBlockID).Return(index, nil)

	s.engine.OnBlockIncorporated(model.BlockFromFlow(incorporatedBlock))

	// matching engine has at least 100ms ticks for processing events
	time.Sleep(1 * time.Second)

	s.core.AssertExpectations(s.T())
}

// TestMultipleProcessingItems tests that the engine queues multiple receipts
// and eventually feeds them into matching.Core for processing
func (s *MatchingEngineSuite) TestMultipleProcessingItems() {
	originID := unittest.IdentifierFixture()
	block := unittest.BlockFixture()

	receipts := make([]*flow.ExecutionReceipt, 20)
	for i := range receipts {
		receipt := unittest.ExecutionReceiptFixture(
			unittest.WithExecutorID(originID),
			unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(block))),
		)
		receipts[i] = receipt
		s.core.On("ProcessReceipt", receipt).Return(nil).Once()
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, receipt := range receipts {
			err := s.engine.Process(channels.ReceiveReceipts, originID, receipt)
			s.Require().NoError(err, "should add receipt and result to mempool if valid")
		}
	}()

	wg.Wait()

	// matching engine has at least 100ms ticks for processing events
	time.Sleep(1 * time.Second)

	s.core.AssertExpectations(s.T())
}

// TestProcessUnsupportedMessageType tests that Process correctly handles a case where invalid message type
// (byzantine message) was submitted from network layer.
func (s *MatchingEngineSuite) TestProcessUnsupportedMessageType() {
	invalidEvent := uint64(42)
	err := s.engine.Process("ch", unittest.IdentifierFixture(), invalidEvent)
	// shouldn't result in error since byzantine inputs are expected
	require.NoError(s.T(), err)
	// Local processing happens only via HandleReceipt, which will log.Fatal on invalid input
}

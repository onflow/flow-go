package matching

import (
	mockconsensus "github.com/onflow/flow-go/engine/consensus/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"os"
	"sync"
	"testing"
	"time"
)

func TestMatchingEngineContext(t *testing.T) {
	suite.Run(t, new(MatchingEngineSuite))
}

type MatchingEngineSuite struct {
	suite.Suite

	core *mockconsensus.MatchingCore

	// Matching Engine
	engine *Engine
}

func (s *MatchingEngineSuite) SetupTest() {
	log := zerolog.New(os.Stderr)
	metrics := metrics.NewNoopCollector()
	me := &mockmodule.Local{}
	net := &mockmodule.Network{}
	s.core = &mockconsensus.MatchingCore{}

	ourNodeID := unittest.IdentifierFixture()
	me.On("NodeID").Return(ourNodeID)

	con := &mocknetwork.Conduit{}
	net.On("Register", mock.Anything, mock.Anything).Return(con, nil).Once()

	var err error
	s.engine, err = NewEngine(log, net, me, metrics, metrics, s.core)
	require.NoError(s.T(), err)

	<-s.engine.Ready()
}

// TestHandleFinalizedBlock tests if finalized block gets processed when send through `Engine`.
// Tests the whole processing pipeline.
func (s *MatchingEngineSuite) TestHandleFinalizedBlock() {

	finalizedBlockID := unittest.IdentifierFixture()
	s.core.On("ProcessFinalizedBlock", finalizedBlockID).Return(nil).Once()
	s.engine.HandleFinalizedBlock(finalizedBlockID)

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
			unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&block))),
		)
		receipts[i] = receipt
		s.core.On("ProcessReceipt", originID, receipt).Return(nil).Once()
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

	wg.Wait()

	// matching engine has at least 100ms ticks for processing events
	time.Sleep(1 * time.Second)

	s.core.AssertExpectations(s.T())
}

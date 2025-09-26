package follower

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	followermock "github.com/onflow/flow-go/engine/common/follower/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	mocknetwork "github.com/onflow/flow-go/network/mock"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestFollowerEngine(t *testing.T) {
	suite.Run(t, new(EngineSuite))
}

// EngineSuite wraps CoreSuite and stores additional state needed for ComplianceEngine specific logic.
type EngineSuite struct {
	suite.Suite

	finalized *flow.Header
	net       *mocknetwork.EngineRegistry
	con       *mocknetwork.Conduit
	me        *module.Local
	headers   *storage.Headers
	core      *followermock.ComplianceCore

	ctx    irrecoverable.SignalerContext
	cancel context.CancelFunc
	errs   <-chan error
	engine *ComplianceEngine
}

func (s *EngineSuite) SetupTest() {

	s.net = mocknetwork.NewEngineRegistry(s.T())
	s.con = mocknetwork.NewConduit(s.T())
	s.me = module.NewLocal(s.T())
	s.headers = storage.NewHeaders(s.T())

	s.core = followermock.NewComplianceCore(s.T())
	s.core.On("Start", mock.Anything).Return().Once()
	unittest.ReadyDoneify(s.core)

	nodeID := unittest.IdentifierFixture()
	s.me.On("NodeID").Return(nodeID).Maybe()

	s.net.On("Register", mock.Anything, mock.Anything).Return(s.con, nil)

	metrics := metrics.NewNoopCollector()
	s.finalized = unittest.BlockHeaderFixture()
	eng, err := NewComplianceLayer(
		unittest.Logger(),
		s.net,
		s.me,
		metrics,
		s.headers,
		s.finalized,
		s.core,
		compliance.DefaultConfig())
	require.NoError(s.T(), err)

	s.engine = eng

	s.ctx, s.cancel, s.errs = irrecoverable.WithSignallerAndCancel(context.Background())
	s.engine.Start(s.ctx)
	unittest.RequireCloseBefore(s.T(), s.engine.Ready(), time.Second, "engine failed to start")
}

// TearDownTest stops the engine and checks there are no errors thrown to the SignallerContext.
func (s *EngineSuite) TearDownTest() {
	s.cancel()
	unittest.RequireCloseBefore(s.T(), s.engine.Done(), time.Second, "engine failed to stop")
	select {
	case err := <-s.errs:
		assert.NoError(s.T(), err)
	default:
	}
}

// TestProcessSyncedBlock checks that processing single synced block results in call to FollowerCore.
func (s *EngineSuite) TestProcessSyncedBlock() {
	block := unittest.BlockWithParentFixture(s.finalized)
	proposal := unittest.ProposalFromBlock(block)

	originID := unittest.IdentifierFixture()
	done := make(chan struct{})
	s.core.On("OnBlockRange", originID, []*flow.Proposal{proposal}).Return(nil).Run(func(_ mock.Arguments) {
		close(done)
	}).Once()

	s.engine.OnSyncedBlocks(flow.Slashable[[]*flow.Proposal]{
		OriginID: originID,
		Message:  []*flow.Proposal{proposal},
	})
	unittest.AssertClosesBefore(s.T(), done, time.Second)
}

// TestProcessGossipedValidBlock check that processing single structurally valid gossiped block results in call to FollowerCore.
func (s *EngineSuite) TestProcessGossipedValidBlock() {
	block := unittest.BlockWithParentFixture(s.finalized)
	proposal := unittest.ProposalFromBlock(block)

	originID := unittest.IdentifierFixture()
	done := make(chan struct{})
	s.core.On("OnBlockRange", originID, []*flow.Proposal{proposal}).Return(nil).Run(func(_ mock.Arguments) {
		close(done)
	}).Once()

	err := s.engine.Process(channels.ReceiveBlocks, originID, proposal)
	require.NoError(s.T(), err)

	unittest.AssertClosesBefore(s.T(), done, time.Second)
}

// TestProcessGossipedInvalidBlock check that processing single structurally invalid gossiped block results in call to FollowerCore.
func (s *EngineSuite) TestProcessGossipedInvalidBlock() {
	block := unittest.BlockWithParentFixture(s.finalized)
	proposal := unittest.ProposalFromBlock(block)
	proposal.ProposerSigData = nil

	originID := unittest.IdentifierFixture()

	err := s.engine.Process(channels.ReceiveBlocks, originID, (*messages.Proposal)(proposal))
	require.NoError(s.T(), err)

	// OnBlockRange should NOT be called for invalid proposal
	s.core.AssertNotCalled(s.T(), "OnBlockRange", mock.Anything, mock.Anything)
}

// TestProcessBlockFromComplianceInterface check that processing single gossiped block using compliance interface results in call to FollowerCore.
func (s *EngineSuite) TestProcessBlockFromComplianceInterface() {
	block := unittest.BlockWithParentFixture(s.finalized)
	proposal := unittest.ProposalFromBlock(block)

	originID := unittest.IdentifierFixture()
	done := make(chan struct{})
	s.core.On("OnBlockRange", originID, []*flow.Proposal{proposal}).Return(nil).Run(func(_ mock.Arguments) {
		close(done)
	}).Once()

	s.engine.OnBlockProposal(flow.Slashable[*flow.Proposal]{
		OriginID: originID,
		Message:  proposal,
	})

	unittest.AssertClosesBefore(s.T(), done, time.Second)
}

// TestProcessBatchOfDisconnectedBlocks tests that processing a batch that consists of one connected range and individual blocks
// results in submitting all of them.
func (s *EngineSuite) TestProcessBatchOfDisconnectedBlocks() {
	originID := unittest.IdentifierFixture()
	blocks := unittest.ProposalChainFixtureFrom(10, s.finalized)
	// drop second block
	blocks = append(blocks[0:1], blocks[2:]...)
	// drop second from end block
	blocks = append(blocks[:len(blocks)-2], blocks[len(blocks)-1])

	var wg sync.WaitGroup
	wg.Add(3)
	s.core.On("OnBlockRange", originID, blocks[0:1]).Run(func(_ mock.Arguments) {
		wg.Done()
	}).Return(nil).Once()
	s.core.On("OnBlockRange", originID, blocks[1:len(blocks)-1]).Run(func(_ mock.Arguments) {
		wg.Done()
	}).Return(nil).Once()
	s.core.On("OnBlockRange", originID, blocks[len(blocks)-1:]).Run(func(_ mock.Arguments) {
		wg.Done()
	}).Return(nil).Once()

	s.engine.OnSyncedBlocks(flow.Slashable[[]*flow.Proposal]{
		OriginID: originID,
		Message:  blocks,
	})
	unittest.RequireReturnsBefore(s.T(), wg.Wait, time.Millisecond*500, "expect to return before timeout")
}

// TestProcessFinalizedBlock tests processing finalized block results in updating last finalized view and propagating it to
// FollowerCore.
// After submitting new finalized block, we check if new batches are filtered based on new finalized view.
func (s *EngineSuite) TestProcessFinalizedBlock() {
	newFinalizedBlock := unittest.BlockHeaderWithParentFixture(s.finalized)

	done := make(chan struct{})
	s.core.On("OnFinalizedBlock", newFinalizedBlock).Run(func(_ mock.Arguments) {
		close(done)
	}).Return(nil).Once()
	s.headers.On("ByBlockID", newFinalizedBlock.ID()).Return(newFinalizedBlock, nil).Once()

	s.engine.OnFinalizedBlock(model.BlockFromFlow(newFinalizedBlock))
	unittest.RequireCloseBefore(s.T(), done, time.Millisecond*500, "expect to close before timeout")

	// check if batch gets filtered out since it's lower than finalized view
	done = make(chan struct{})
	block := unittest.BlockWithParentFixture(s.finalized)
	block.View = newFinalizedBlock.View - 1 // use block view lower than new latest finalized view

	proposal := unittest.ProposalFromBlock(block)

	// use metrics mock to track that we have indeed processed the message, and the batch was filtered out since it was
	// lower than finalized height
	metricsMock := module.NewEngineMetrics(s.T())
	metricsMock.On("MessageReceived", mock.Anything, metrics.MessageSyncedBlocks).Return().Once()
	metricsMock.On("MessageHandled", mock.Anything, metrics.MessageSyncedBlocks).Run(func(_ mock.Arguments) {
		close(done)
	}).Return().Once()
	s.engine.engMetrics = metricsMock

	s.engine.OnSyncedBlocks(flow.Slashable[[]*flow.Proposal]{
		OriginID: unittest.IdentifierFixture(),
		Message:  []*flow.Proposal{proposal},
	})
	unittest.RequireCloseBefore(s.T(), done, time.Millisecond*500, "expect to close before timeout")
	// check if message wasn't buffered in internal channel
	select {
	case <-s.engine.pendingConnectedBlocksChan:
		s.Fail("channel has to be empty at this stage")
	default:

	}
}

package follower_test

import (
	"context"
	"github.com/onflow/flow-go/model/events"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine/common/follower"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	realstorage "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	net       *mocknetwork.Network
	con       *mocknetwork.Conduit
	me        *module.Local
	cleaner   *storage.Cleaner
	headers   *storage.Headers
	payloads  *storage.Payloads
	state     *protocol.MutableState
	snapshot  *protocol.Snapshot
	cache     *module.PendingBlockBuffer
	follower  *module.HotStuffFollower
	sync      *module.BlockRequester
	validator *hotstuff.Validator

	ctx    irrecoverable.SignalerContext
	cancel context.CancelFunc
	errs   <-chan error
	engine *follower.Engine
}

func (s *Suite) SetupTest() {

	s.net = mocknetwork.NewNetwork(s.T())
	s.con = mocknetwork.NewConduit(s.T())
	s.me = module.NewLocal(s.T())
	s.cleaner = storage.NewCleaner(s.T())
	s.headers = storage.NewHeaders(s.T())
	s.payloads = storage.NewPayloads(s.T())
	s.state = protocol.NewMutableState(s.T())
	s.snapshot = protocol.NewSnapshot(s.T())
	s.cache = module.NewPendingBlockBuffer(s.T())
	s.follower = module.NewHotStuffFollower(s.T())
	s.validator = hotstuff.NewValidator(s.T())
	s.sync = module.NewBlockRequester(s.T())

	nodeID := unittest.IdentifierFixture()
	s.me.On("NodeID").Return(nodeID).Maybe()

	s.net.On("Register", mock.Anything, mock.Anything).Return(s.con, nil)
	s.cleaner.On("RunGC").Return().Maybe()
	s.state.On("Final").Return(s.snapshot).Maybe()
	s.cache.On("PruneByView", mock.Anything).Return().Maybe()
	s.cache.On("Size", mock.Anything).Return(uint(0)).Maybe()

	metrics := metrics.NewNoopCollector()
	eng, err := follower.New(
		unittest.Logger(),
		s.net,
		s.me,
		metrics,
		metrics,
		s.cleaner,
		s.headers,
		s.payloads,
		s.state,
		s.cache,
		s.follower,
		s.validator,
		s.sync,
		trace.NewNoopTracer())
	require.Nil(s.T(), err)

	s.engine = eng

	s.follower.On("Start", mock.Anything).Once()
	unittest.ReadyDoneify(s.follower)

	s.ctx, s.cancel, s.errs = irrecoverable.WithSignallerAndCancel(context.Background())
	s.engine.Start(s.ctx)
	unittest.RequireCloseBefore(s.T(), s.engine.Ready(), time.Second, "engine failed to start")
}

// TearDownTest stops the engine and checks there are no errors thrown to the SignallerContext.
func (s *Suite) TearDownTest() {
	s.cancel()
	unittest.RequireCloseBefore(s.T(), s.engine.Done(), time.Second, "engine failed to stop")
	select {
	case err := <-s.errs:
		assert.NoError(s.T(), err)
	default:
	}
}

func TestFollower(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (s *Suite) TestHandlePendingBlock() {

	originID := unittest.IdentifierFixture()
	head := unittest.BlockFixture()
	block := unittest.BlockFixture()

	head.Header.Height = 10
	block.Header.Height = 12

	// not in cache
	s.cache.On("ByID", block.ID()).Return(nil, false).Once()
	s.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound).Once()

	// don't return the parent when requested
	s.snapshot.On("Head").Return(head.Header, nil)
	s.cache.On("ByID", block.Header.ParentID).Return(nil, false).Once()
	s.headers.On("ByBlockID", block.Header.ParentID).Return(nil, realstorage.ErrNotFound).Once()

	done := make(chan struct{})
	s.cache.On("Add", mock.Anything, mock.Anything).Return(true).Once()
	s.sync.On("RequestBlock", block.Header.ParentID, block.Header.Height-1).Run(func(_ mock.Arguments) {
		close(done)
	}).Return().Once()

	// submit the block
	proposal := unittest.ProposalFromBlock(&block)
	err := s.engine.Process(channels.ReceiveBlocks, originID, proposal)
	assert.Nil(s.T(), err)

	unittest.AssertClosesBefore(s.T(), done, time.Second)
	s.follower.AssertNotCalled(s.T(), "SubmitProposal", mock.Anything)
}

func (s *Suite) TestHandleProposal() {

	originID := unittest.IdentifierFixture()
	parent := unittest.BlockFixture()
	block := unittest.BlockFixture()

	parent.Header.Height = 10
	block.Header.Height = 11
	block.Header.ParentID = parent.ID()

	// not in cache
	s.cache.On("ByID", block.ID()).Return(nil, false).Once()
	s.cache.On("ByID", block.Header.ParentID).Return(nil, false).Once()
	s.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound).Once()

	done := make(chan struct{})
	hotstuffProposal := model.ProposalFromFlow(block.Header)

	// the parent is the last finalized state
	s.snapshot.On("Head").Return(parent.Header, nil)
	// the block passes hotstuff validation
	s.validator.On("ValidateProposal", hotstuffProposal).Return(nil)
	// we should be able to extend the state with the block
	s.state.On("Extend", mock.Anything, &block).Return(nil).Once()
	// we should be able to get the parent header by its ID
	s.headers.On("ByBlockID", block.Header.ParentID).Return(parent.Header, nil).Once()
	// we do not have any children cached
	s.cache.On("ByParentID", block.ID()).Return(nil, false)
	// the proposal should be forwarded to the follower
	s.follower.On("SubmitProposal", hotstuffProposal).Run(func(_ mock.Arguments) {
		close(done)
	}).Once()

	// submit the block
	proposal := unittest.ProposalFromBlock(&block)
	err := s.engine.Process(channels.ReceiveBlocks, originID, proposal)
	assert.Nil(s.T(), err)
	unittest.AssertClosesBefore(s.T(), done, time.Second)
}

func (s *Suite) TestHandleProposalSkipProposalThreshold() {

	// mock latest finalized state
	final := unittest.BlockHeaderFixture()
	s.snapshot.On("Head").Return(final, nil)

	originID := unittest.IdentifierFixture()
	block := unittest.BlockFixture()

	block.Header.Height = final.Height + compliance.DefaultConfig().SkipNewProposalsThreshold + 1

	done := make(chan struct{})

	// not in cache or storage
	s.cache.On("ByID", block.ID()).Return(nil, false).Once()
	s.headers.On("ByBlockID", block.ID()).Run(func(_ mock.Arguments) {
		close(done)
	}).Return(nil, realstorage.ErrNotFound).Once()

	// submit the block
	proposal := unittest.ProposalFromBlock(&block)
	err := s.engine.Process(channels.ReceiveBlocks, originID, proposal)
	assert.NoError(s.T(), err)
	unittest.AssertClosesBefore(s.T(), done, time.Second)

	// block should be dropped - not added to state or cache
	s.state.AssertNotCalled(s.T(), "Extend", mock.Anything)
	s.cache.AssertNotCalled(s.T(), "Add", originID, mock.Anything)
}

// TestHandleProposalWithPendingChildren tests processing a block which has a pending
// child cached.
//   - the block should be processed
//   - the cached child block should also be processed
func (s *Suite) TestHandleProposalWithPendingChildren() {

	originID := unittest.IdentifierFixture()
	parent := unittest.BlockFixture()                       // already processed and incorporated block
	block := unittest.BlockWithParentFixture(parent.Header) // block which is passed as input to the engine
	child := unittest.BlockWithParentFixture(block.Header)  // block which is already cached

	done := make(chan struct{})
	hotstuffProposal := model.ProposalFromFlow(block.Header)
	childHotstuffProposal := model.ProposalFromFlow(child.Header)

	// the parent is the last finalized state
	s.snapshot.On("Head").Return(parent.Header, nil)

	s.cache.On("ByID", mock.Anything).Return(nil, false)
	// first time calling, assume it's not there
	s.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound).Once()
	// both blocks pass HotStuff validation
	s.validator.On("ValidateProposal", hotstuffProposal).Return(nil)
	s.validator.On("ValidateProposal", childHotstuffProposal).Return(nil)
	// should extend state with the input block, and the child
	s.state.On("Extend", mock.Anything, block).Return(nil).Once()
	s.state.On("Extend", mock.Anything, child).Return(nil).Once()
	// we have already received and stored the parent
	s.headers.On("ByBlockID", parent.ID()).Return(parent.Header, nil).Once()
	// should submit to follower
	s.follower.On("SubmitProposal", hotstuffProposal).Once()
	s.follower.On("SubmitProposal", childHotstuffProposal).Run(func(_ mock.Arguments) {
		close(done)
	}).Once()

	// we have one pending child cached
	pending := []*flow.PendingBlock{
		{
			OriginID: originID,
			Header:   child.Header,
			Payload:  child.Payload,
		},
	}
	s.cache.On("ByParentID", block.ID()).Return(pending, true).Once()
	s.cache.On("ByParentID", child.ID()).Return(nil, false).Once()
	s.cache.On("DropForParent", block.ID()).Once()

	// submit the block proposal
	proposal := unittest.ProposalFromBlock(block)
	err := s.engine.Process(channels.ReceiveBlocks, originID, proposal)
	assert.NoError(s.T(), err)
	unittest.AssertClosesBefore(s.T(), done, time.Second)
}

// TestProcessSyncedBlock checks if processing synced block using unsafe API results in error.
// All blocks from sync engine should be sent through dedicated compliance API.
func (s *Suite) TestProcessSyncedBlock() {
	parent := unittest.BlockFixture()
	block := unittest.BlockFixture()

	parent.Header.Height = 10
	block.Header.Height = 11
	block.Header.ParentID = parent.ID()

	syncedBlock := &events.SyncedBlock{
		OriginID: unittest.IdentifierFixture(),
		Block:    &block,
	}

	// using unsafe interface should result in error
	err := s.engine.Process(channels.ReceiveBlocks, s.me.NodeID(), syncedBlock)
	require.Error(s.T(), err)

	// not in cache
	s.cache.On("ByID", block.ID()).Return(nil, false).Once()
	s.cache.On("ByID", block.Header.ParentID).Return(nil, false).Once()
	s.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound).Once()

	done := make(chan struct{})
	hotstuffProposal := model.ProposalFromFlow(block.Header)

	// the parent is the last finalized state
	s.snapshot.On("Head").Return(parent.Header, nil)
	// the block passes hotstuff validation
	s.validator.On("ValidateProposal", hotstuffProposal).Return(nil)
	// we should be able to extend the state with the block
	s.state.On("Extend", mock.Anything, &block).Return(nil).Once()
	// we should be able to get the parent header by its ID
	s.headers.On("ByBlockID", block.Header.ParentID).Return(parent.Header, nil).Once()
	// we do not have any children cached
	s.cache.On("ByParentID", block.ID()).Return(nil, false)
	// the proposal should be forwarded to the follower
	s.follower.On("SubmitProposal", hotstuffProposal).Run(func(_ mock.Arguments) {
		close(done)
	}).Once()

	s.engine.OnSyncedBlock(syncedBlock)
	unittest.AssertClosesBefore(s.T(), done, time.Second)
}

package follower_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/follower"
	"github.com/onflow/flow-go/model/flow"
	metrics "github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	realstorage "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	net      *mocknetwork.Network
	con      *mocknetwork.Conduit
	me       *module.Local
	cleaner  *storage.Cleaner
	headers  *storage.Headers
	payloads *storage.Payloads
	state    *protocol.MutableState
	snapshot *protocol.Snapshot
	cache    *module.PendingBlockBuffer
	follower *module.HotStuffFollower

	engine *follower.Engine
	sync   *module.BlockRequester
}

func (suite *Suite) SetupTest() {

	suite.net = new(mocknetwork.Network)
	suite.con = new(mocknetwork.Conduit)
	suite.me = new(module.Local)
	suite.cleaner = new(storage.Cleaner)
	suite.headers = new(storage.Headers)
	suite.payloads = new(storage.Payloads)
	suite.state = new(protocol.MutableState)
	suite.snapshot = new(protocol.Snapshot)
	suite.cache = new(module.PendingBlockBuffer)
	suite.follower = new(module.HotStuffFollower)
	suite.sync = new(module.BlockRequester)

	suite.net.On("Register", mock.Anything, mock.Anything).Return(suite.con, nil)
	suite.cleaner.On("RunGC").Return()
	suite.headers.On("Store", mock.Anything).Return(nil)
	suite.payloads.On("Store", mock.Anything, mock.Anything).Return(nil)
	suite.state.On("Final").Return(suite.snapshot)
	suite.cache.On("PruneByView", mock.Anything).Return()
	suite.cache.On("Size", mock.Anything).Return(uint(0))

	metrics := metrics.NewNoopCollector()
	eng, err := follower.New(zerolog.Logger{},
		suite.net,
		suite.me,
		metrics,
		metrics,
		suite.cleaner,
		suite.headers,
		suite.payloads,
		suite.state,
		suite.cache,
		suite.follower,
		suite.sync,
		trace.NewNoopTracer())
	require.Nil(suite.T(), err)

	suite.engine = eng
}

func TestFollower(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) TestHandlePendingBlock() {

	originID := unittest.IdentifierFixture()
	head := unittest.BlockFixture()
	block := unittest.BlockFixture()

	head.Header.Height = 10
	block.Header.Height = 12

	// not in cache
	suite.cache.On("ByID", block.ID()).Return(nil, false).Once()
	suite.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound).Once()

	// don't return the parent when requested
	suite.snapshot.On("Head").Return(head.Header, nil).Once()
	suite.cache.On("ByID", block.Header.ParentID).Return(nil, false).Once()
	suite.headers.On("ByBlockID", block.Header.ParentID).Return(nil, realstorage.ErrNotFound).Once()

	suite.cache.On("Add", mock.Anything, mock.Anything).Return(true).Once()
	suite.sync.On("RequestBlock", block.Header.ParentID).Return().Once()

	// submit the block
	proposal := unittest.ProposalFromBlock(&block)
	err := suite.engine.Process(engine.ReceiveBlocks, originID, proposal)
	assert.Nil(suite.T(), err)

	suite.follower.AssertNotCalled(suite.T(), "SubmitProposal", mock.Anything)
	suite.cache.AssertExpectations(suite.T())
	suite.con.AssertExpectations(suite.T())
}

func (suite *Suite) TestHandleProposal() {

	originID := unittest.IdentifierFixture()
	parent := unittest.BlockFixture()
	block := unittest.BlockFixture()

	parent.Header.Height = 10
	block.Header.Height = 11
	block.Header.ParentID = parent.ID()

	// not in cache
	suite.cache.On("ByID", block.ID()).Return(nil, false).Once()
	suite.cache.On("ByID", block.Header.ParentID).Return(nil, false).Once()
	suite.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound).Once()

	// the parent is the last finalized state
	suite.snapshot.On("Head").Return(parent.Header, nil).Once()
	// we should be able to extend the state with the block
	suite.state.On("Extend", mock.Anything, &block).Return(nil).Once()
	// we should be able to get the parent header by its ID
	suite.headers.On("ByBlockID", block.Header.ParentID).Return(parent.Header, nil).Twice()
	// we do not have any children cached
	suite.cache.On("ByParentID", block.ID()).Return(nil, false)
	// the proposal should be forwarded to the follower
	suite.follower.On("SubmitProposal", block.Header, parent.Header.View).Once()

	// submit the block
	proposal := unittest.ProposalFromBlock(&block)
	err := suite.engine.Process(engine.ReceiveBlocks, originID, proposal)
	assert.Nil(suite.T(), err)

	suite.follower.AssertExpectations(suite.T())
}

func (suite *Suite) TestHandleProposalWithPendingChildren() {

	originID := unittest.IdentifierFixture()
	parent := unittest.BlockFixture()
	block := unittest.BlockFixture()
	child := unittest.BlockFixture()

	parent.Header.Height = 9
	block.Header.Height = 10
	child.Header.Height = 11

	block.Header.ParentID = parent.ID()
	child.Header.ParentID = block.ID()

	// the parent is the last finalized state
	suite.snapshot.On("Head").Return(parent.Header, nil).Once()
	suite.snapshot.On("Head").Return(block.Header, nil).Once()

	// both parent and child not in cache
	// suite.cache.On("ByID", child.ID()).Return(nil, false).Once()
	suite.cache.On("ByID", block.ID()).Return(nil, false).Once()
	suite.cache.On("ByID", block.Header.ParentID).Return(nil, false).Once()
	// first time calling, assume it's not there
	suite.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound).Once()
	// should extend state with new block
	suite.state.On("Extend", mock.Anything, &block).Return(nil).Once()
	suite.state.On("Extend", mock.Anything, &child).Return(nil).Once()
	// we have already received and stored the parent
	suite.headers.On("ByBlockID", parent.ID()).Return(parent.Header, nil)
	suite.headers.On("ByBlockID", block.ID()).Return(block.Header, nil).Once()
	// should submit to follower
	suite.follower.On("SubmitProposal", block.Header, parent.Header.View).Once()
	suite.follower.On("SubmitProposal", child.Header, block.Header.View).Once()

	// we have one pending child cached
	pending := []*flow.PendingBlock{
		{
			OriginID: originID,
			Header:   child.Header,
			Payload:  child.Payload,
		},
	}
	suite.cache.On("ByParentID", block.ID()).Return(pending, true)
	suite.cache.On("ByParentID", child.ID()).Return(nil, false)
	suite.cache.On("DropForParent", block.ID()).Once()

	// submit the block proposal
	proposal := unittest.ProposalFromBlock(&block)
	err := suite.engine.Process(engine.ReceiveBlocks, originID, proposal)
	assert.Nil(suite.T(), err)

	suite.follower.AssertExpectations(suite.T())
}

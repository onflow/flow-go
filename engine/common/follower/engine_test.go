package follower_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine/common/follower"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	realstorage "github.com/dapperlabs/flow-go/storage"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	net      *module.Network
	con      *network.Conduit
	me       *module.Local
	state    *protocol.State
	mutator  *protocol.Mutator
	snapshot *protocol.Snapshot
	headers  *storage.Headers
	payloads *storage.Payloads
	cache    *module.PendingBlockBuffer
	follower *module.HotStuffFollower

	engine *follower.Engine
	sync   *module.Synchronization
}

func (suite *Suite) SetupTest() {

	suite.net = new(module.Network)
	suite.con = new(network.Conduit)
	suite.me = new(module.Local)
	suite.state = new(protocol.State)
	suite.mutator = new(protocol.Mutator)
	suite.snapshot = new(protocol.Snapshot)
	suite.headers = new(storage.Headers)
	suite.payloads = new(storage.Payloads)
	suite.cache = new(module.PendingBlockBuffer)
	suite.follower = new(module.HotStuffFollower)
	suite.sync = new(module.Synchronization)

	suite.net.On("Register", mock.Anything, mock.Anything).Return(suite.con, nil)
	suite.state.On("Mutate").Return(suite.mutator)
	suite.state.On("Final").Return(suite.snapshot)
	suite.headers.On("Store", mock.Anything).Return(nil)
	suite.payloads.On("Store", mock.Anything, mock.Anything).Return(nil)

	eng, err := follower.New(zerolog.Logger{}, suite.net, suite.me, suite.state, suite.headers, suite.payloads, suite.cache, suite.follower)
	require.Nil(suite.T(), err)
	eng.WithSynchronization(suite.sync)

	suite.engine = eng
}

func TestFollower(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) TestHandlePendingBlock() {

	originID := unittest.IdentifierFixture()
	head := unittest.BlockFixture()
	block := unittest.BlockFixture()

	head.Height = 10
	block.Height = 12

	// don't return the parent when requested
	suite.snapshot.On("Head").Return(&head.Header, nil).Once()
	suite.cache.On("ByID", block.ParentID).Return(nil, false).Once()
	suite.cache.On("Add", mock.Anything).Return(true).Once()
	suite.sync.On("RequestBlock", block.ParentID).Return().Once()
	suite.headers.On("ByBlockID", mock.Anything).Return(nil, realstorage.ErrNotFound).Once()

	// submit the block
	proposal := messages.BlockProposal{
		Header:  &block.Header,
		Payload: &block.Payload,
	}
	err := suite.engine.Process(originID, &proposal)
	assert.Nil(suite.T(), err)

	suite.follower.AssertNotCalled(suite.T(), "SubmitProposal", mock.Anything)
	suite.cache.AssertExpectations(suite.T())
	suite.con.AssertExpectations(suite.T())
}

func (suite *Suite) TestHandleProposal() {

	originID := unittest.IdentifierFixture()
	parent := unittest.BlockFixture()
	block := unittest.BlockFixture()

	parent.Height = 10
	block.Height = 11
	block.ParentID = parent.ID()

	// the parent is the last finalized state
	suite.snapshot.On("Head").Return(&parent.Header, nil).Once()
	// we should be able to extend the state with the block
	suite.mutator.On("Extend", block.ID()).Return(nil).Once()
	// we should be able to get the parent header by its ID
	suite.headers.On("ByBlockID", block.ParentID).Return(&parent.Header, nil).Once()
	// we do not have any children cached
	suite.cache.On("ByParentID", block.ID()).Return(nil, false)
	// the proposal should be forwarded to the follower
	suite.follower.On("SubmitProposal", &block.Header, parent.View).Once()

	suite.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound).Once()

	// submit the block
	proposal := messages.BlockProposal{
		Header:  &block.Header,
		Payload: &block.Payload,
	}
	err := suite.engine.Process(originID, &proposal)
	assert.Nil(suite.T(), err)

	suite.follower.AssertExpectations(suite.T())
}

func (suite *Suite) TestHandleProposalWithPendingChildren() {

	originID := unittest.IdentifierFixture()
	parent := unittest.BlockFixture()
	block := unittest.BlockFixture()
	child := unittest.BlockFixture()

	parent.Height = 9
	block.Height = 10
	child.Height = 11

	block.ParentID = parent.ID()
	child.ParentID = block.ID()

	// the parent is the last finalized state
	suite.snapshot.On("Head").Return(&parent.Header, nil).Once()
	suite.snapshot.On("Head").Return(&block.Header, nil).Once()
	// should extend state with new block
	suite.mutator.On("Extend", block.ID()).Return(nil).Once()
	suite.mutator.On("Extend", child.ID()).Return(nil).Once()
	// we have already received and stored the parent
	suite.headers.On("ByBlockID", parent.ID()).Return(&parent.Header, nil)
	suite.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound).Once()
	suite.headers.On("ByBlockID", block.ID()).Return(&block.Header, nil).Once()
	suite.headers.On("ByBlockID", child.ID()).Return(nil, realstorage.ErrNotFound).Once()
	// should submit to follower
	suite.follower.On("SubmitProposal", &block.Header, parent.View).Once()
	suite.follower.On("SubmitProposal", &child.Header, block.View).Once()

	// we have one pending child cached
	pending := []*flow.PendingBlock{
		{
			OriginID: originID,
			Header:   &child.Header,
			Payload:  &child.Payload,
		},
	}
	suite.cache.On("ByParentID", block.ID()).Return(pending, true)
	suite.cache.On("ByParentID", child.ID()).Return(nil, false)
	suite.cache.On("DropForParent", block.ID()).Once()

	// submit the block proposal
	proposal := messages.BlockProposal{
		Header:  &block.Header,
		Payload: &block.Payload,
	}
	err := suite.engine.Process(originID, &proposal)
	assert.Nil(suite.T(), err)

	suite.follower.AssertExpectations(suite.T())
}

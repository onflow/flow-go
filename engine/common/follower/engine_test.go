package follower_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine/common/follower"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
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
	headers  *storage.Headers
	payloads *storage.Payloads
	cache    *module.PendingBlockBuffer
	follower *module.HotStuffFollower

	engine *follower.Engine
}

func (suite *Suite) SetupTest() {

	suite.net = new(module.Network)
	suite.con = new(network.Conduit)
	suite.me = new(module.Local)
	suite.state = new(protocol.State)
	suite.mutator = new(protocol.Mutator)
	suite.headers = new(storage.Headers)
	suite.payloads = new(storage.Payloads)
	suite.cache = new(module.PendingBlockBuffer)
	suite.follower = new(module.HotStuffFollower)

	suite.net.On("Register", mock.Anything, mock.Anything).Return(suite.con, nil)
	suite.state.On("Mutate").Return(suite.mutator)
	suite.headers.On("Store", mock.Anything, mock.Anything).Return(nil)
	suite.payloads.On("Store", mock.Anything, mock.Anything).Return(nil)

	eng, err := follower.New(zerolog.Logger{}, suite.net, suite.me, suite.state, suite.headers, suite.payloads, suite.cache, suite.follower)
	require.Nil(suite.T(), err)

	suite.engine = eng
}

func TestFollower(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) TestHandlePendingBlock() {

	originID := unittest.IdentifierFixture()
	block := unittest.BlockFixture()

	// don't return the parent when requested
	suite.headers.On("ByBlockID", block.ParentID).Return(nil, realstorage.ErrNotFound)
	suite.cache.On("Add", mock.Anything).Return(true).Once()
	suite.con.On("Submit", mock.Anything, mock.Anything).Return(nil).Once()

	err := suite.engine.Process(originID, &block)
	assert.Nil(suite.T(), err)

	suite.follower.AssertNotCalled(suite.T(), "SubmitProposal", mock.Anything)
	suite.cache.AssertExpectations(suite.T())
	suite.con.AssertExpectations(suite.T())
}

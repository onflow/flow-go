package multiplexer_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/common/multiplexer"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/mocknetwork"
)

type Suite struct {
	suite.Suite

	net *module.Network
	con *mocknetwork.Conduit
	me  *module.Local

	engine *multiplexer.Engine
}

func (suite *Suite) SetupTest() {
	suite.net = new(module.Network)
	suite.con = new(mocknetwork.Conduit)
	suite.me = new(module.Local)

	suite.net.On("Register", mock.Anything, mock.Anything).Return(suite.con, nil)
	// TODO

	eng, err := multiplexer.New(zerolog.Logger{}, suite.net, suite.me)
	require.Nil(suite.T(), err)

	suite.engine = eng
}

func TestMultiplexer(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) TestHappyPath() {
	// TODO: test happy path
}

func (suite *Suite) TestDownstreamEngineFailure() {
	// TODO: test failure in downstream engine
}

func (suite *Suite) TestProcessUnregisteredChannel() {
	// TODO: test processing message from channel that has no subscriptions
}

func (suite *Suite) TestDuplicateRegistrations() {
	// TODO: test registering same engine twice on same channel
}

func (suite *Suite) TestReadyDone() {
	// TODO: test Ready and Done
}

package multiplexer_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/common/multiplexer"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
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
	id := unittest.IdentifierFixture()
	event := struct {
		foo string
	}{
		foo: "bar",
	}

	chan1 := network.Channel("test-chan-1")
	chan2 := network.Channel("test-chan-2")
	chan3 := network.Channel("test-chan-3")

	engine1 := new(mocknetwork.Engine)
	engine2 := new(mocknetwork.Engine)
	engine3 := new(mocknetwork.Engine)

	_, err := suite.engine.Register(chan1, engine1)
	suite.Assert().Nil(err)
	_, err = suite.engine.Register(chan1, engine2)
	suite.Assert().Nil(err)

	_, err = suite.engine.Register(chan2, engine2)
	suite.Assert().Nil(err)
	_, err = suite.engine.Register(chan2, engine3)
	suite.Assert().Nil(err)

	_, err = suite.engine.Register(chan3, engine1)
	suite.Assert().Nil(err)
	_, err = suite.engine.Register(chan3, engine2)
	suite.Assert().Nil(err)
	_, err = suite.engine.Register(chan3, engine3)
	suite.Assert().Nil(err)

	// Message sent on chan1 should be delivered to engine1 and engine2

	engine1.On("Process", chan1, id, event).Return(nil).Once()
	engine2.On("Process", chan1, id, event).Return(nil).Once()

	err = suite.engine.Process(chan1, id, event)
	suite.Assert().Nil(err)

	engine1.AssertNumberOfCalls(suite.T(), "Process", 1)
	engine2.AssertNumberOfCalls(suite.T(), "Process", 1)
	engine3.AssertNumberOfCalls(suite.T(), "Process", 0)

	engine1.AssertExpectations(suite.T())
	engine2.AssertExpectations(suite.T())
	engine3.AssertExpectations(suite.T())

	// Message sent on chan2 should be delivered to engine2 and engine3

	engine2.On("Process", chan2, id, event).Return(nil).Once()
	engine3.On("Process", chan2, id, event).Return(nil).Once()

	err = suite.engine.Process(chan2, id, event)
	suite.Assert().Nil(err)

	engine1.AssertNumberOfCalls(suite.T(), "Process", 1)
	engine2.AssertNumberOfCalls(suite.T(), "Process", 2)
	engine3.AssertNumberOfCalls(suite.T(), "Process", 1)

	engine1.AssertExpectations(suite.T())
	engine2.AssertExpectations(suite.T())
	engine3.AssertExpectations(suite.T())

	// Message sent on chan3 should be delivered to all engines

	engine1.On("Process", chan3, id, event).Return(nil).Once()
	engine2.On("Process", chan3, id, event).Return(nil).Once()
	engine3.On("Process", chan3, id, event).Return(nil).Once()

	err = suite.engine.Process(chan3, id, event)
	suite.Assert().Nil(err)

	engine1.AssertNumberOfCalls(suite.T(), "Process", 2)
	engine2.AssertNumberOfCalls(suite.T(), "Process", 3)
	engine3.AssertNumberOfCalls(suite.T(), "Process", 2)

	engine1.AssertExpectations(suite.T())
	engine2.AssertExpectations(suite.T())
	engine3.AssertExpectations(suite.T())
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

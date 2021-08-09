package network_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	splitternetwork "github.com/onflow/flow-go/engine/common/splitter/network"
	"github.com/onflow/flow-go/module"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

func getEvent() interface{} {
	return struct {
		foo string
	}{
		foo: "bar",
	}
}

type Suite struct {
	suite.Suite

	con     *mocknetwork.Conduit
	net     *splitternetwork.Network
	engines map[network.Channel]module.Engine
}

func (suite *Suite) SetupTest() {
	net := new(mocknetwork.ReadyDoneAwareNetwork)
	suite.con = new(mocknetwork.Conduit)
	suite.engines = make(map[network.Channel]module.Engine)

	net.On("Register", mock.AnythingOfType("network.Channel"), mock.Anything).Run(func(args mock.Arguments) {
		channel, _ := args.Get(0).(network.Channel)
		engine, ok := args.Get(1).(module.Engine)
		suite.Assert().True(ok)

		suite.engines[channel] = engine
	}).Return(suite.con, nil)

	splitterNet := splitternetwork.NewNetwork(net, zerolog.Logger{})

	suite.net = splitterNet
}

func TestSplitterNetwork(t *testing.T) {
	suite.Run(t, new(Suite))
}

// TestHappyPath tests a basic scenario with three channels and three engines
func (suite *Suite) TestHappyPath() {
	id := unittest.IdentifierFixture()
	event := getEvent()

	chan1 := network.Channel("test-chan-1")
	chan2 := network.Channel("test-chan-2")
	chan3 := network.Channel("test-chan-3")

	engine1 := new(mockmodule.Engine)
	engine2 := new(mockmodule.Engine)
	engine3 := new(mockmodule.Engine)

	con, err := suite.net.Register(chan1, engine1)
	suite.Assert().Nil(err)
	suite.Assert().Equal(suite.con, con)
	con, err = suite.net.Register(chan1, engine2)
	suite.Assert().Nil(err)
	suite.Assert().Equal(suite.con, con)

	con, err = suite.net.Register(chan2, engine2)
	suite.Assert().Nil(err)
	suite.Assert().Equal(suite.con, con)
	con, err = suite.net.Register(chan2, engine3)
	suite.Assert().Nil(err)
	suite.Assert().Equal(suite.con, con)

	con, err = suite.net.Register(chan3, engine1)
	suite.Assert().Nil(err)
	suite.Assert().Equal(suite.con, con)
	con, err = suite.net.Register(chan3, engine2)
	suite.Assert().Nil(err)
	suite.Assert().Equal(suite.con, con)
	con, err = suite.net.Register(chan3, engine3)
	suite.Assert().Nil(err)
	suite.Assert().Equal(suite.con, con)

	// Message sent on chan1 should be delivered to engine1 and engine2

	engine1.On("Process", chan1, id, event).Return(nil).Once()
	engine2.On("Process", chan1, id, event).Return(nil).Once()

	splitter, ok := suite.engines[chan1]
	suite.Assert().True(ok)

	err = splitter.Process(chan1, id, event)
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

	splitter, ok = suite.engines[chan2]
	suite.Assert().True(ok)

	err = splitter.Process(chan2, id, event)
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

	splitter, ok = suite.engines[chan3]
	suite.Assert().True(ok)

	err = splitter.Process(chan3, id, event)
	suite.Assert().Nil(err)

	engine1.AssertNumberOfCalls(suite.T(), "Process", 2)
	engine2.AssertNumberOfCalls(suite.T(), "Process", 3)
	engine3.AssertNumberOfCalls(suite.T(), "Process", 2)

	engine1.AssertExpectations(suite.T())
	engine2.AssertExpectations(suite.T())
	engine3.AssertExpectations(suite.T())
}

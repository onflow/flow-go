package unstaked_test

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/unstaked"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
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
	net          module.Network
	stakedNodeID flow.Identifier
	unstakedNet  *unstaked.UnstakedNetwork
	con          *mocknetwork.Conduit
	engine       module.Engine
}

func TestUnstakedNetwork(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	net := new(mockmodule.Network)
	suite.net = net
	suite.con = new(mocknetwork.Conduit)
	suite.stakedNodeID = unittest.IdentifierFixture()
	suite.unstakedNet = unstaked.NewUnstakedNetwork(suite.net, suite.stakedNodeID)
	suite.engine = new(mockmodule.Engine)

	net.On("Register", mock.AnythingOfType("network.Channel"), mock.Anything).Return(suite.con, nil)
}

func (suite *Suite) TestUnicast() {
	channel := network.Channel("test-channel")
	targetID := unittest.IdentifierFixture()
	event := getEvent()

	con, err := suite.unstakedNet.Register(channel, suite.engine)
	suite.Assert().NoError(err)

	suite.con.On("Unicast", event, suite.stakedNodeID).Return(nil).Once()

	err = con.Unicast(event, targetID)
	suite.Assert().NoError(err)

	suite.con.AssertNumberOfCalls(suite.T(), "Unicast", 1)
	suite.con.AssertExpectations(suite.T())
}

func (suite *Suite) TestPublish() {
	channel := network.Channel("test-channel")
	targetIDs := make([]flow.Identifier, 10)

	for i := 0; i < 10; i++ {
		targetIDs = append(targetIDs, unittest.IdentifierFixture())
	}

	event := getEvent()

	con, err := suite.unstakedNet.Register(channel, suite.engine)
	suite.Assert().NoError(err)

	suite.con.On("Unicast", event, suite.stakedNodeID).Return(nil).Once()

	err = con.Publish(event, targetIDs...)
	suite.Assert().NoError(err)

	suite.con.AssertNumberOfCalls(suite.T(), "Unicast", 1)
	suite.con.AssertExpectations(suite.T())
}

func (suite *Suite) TestMulticast() {
	channel := network.Channel("test-channel")
	targetIDs := make([]flow.Identifier, 10)

	for i := 0; i < 10; i++ {
		targetIDs = append(targetIDs, unittest.IdentifierFixture())
	}

	event := getEvent()

	con, err := suite.unstakedNet.Register(channel, suite.engine)
	suite.Assert().NoError(err)

	suite.con.On("Unicast", event, suite.stakedNodeID).Return(nil).Once()

	err = con.Multicast(event, 5, targetIDs...)
	suite.Assert().NoError(err)

	suite.con.AssertNumberOfCalls(suite.T(), "Unicast", 1)
	suite.con.AssertExpectations(suite.T())
}

func (suite *Suite) TestClose() {
	channel := network.Channel("test-channel")

	con, err := suite.unstakedNet.Register(channel, suite.engine)
	suite.Assert().NoError(err)

	suite.con.On("Close").Return(nil).Once()

	err = con.Close()
	suite.Assert().NoError(err)

	suite.con.AssertNumberOfCalls(suite.T(), "Close", 1)
	suite.con.AssertExpectations(suite.T())
}

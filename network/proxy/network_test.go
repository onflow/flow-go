package proxy_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/proxy"
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
	net          network.Network
	targetNodeID flow.Identifier
	proxyNet     *proxy.ProxyNetwork
	con          *mocknetwork.Conduit
	engine       network.Engine
}

func TestProxyNetwork(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	net := new(mocknetwork.Network)
	suite.net = net
	suite.con = new(mocknetwork.Conduit)
	suite.targetNodeID = unittest.IdentifierFixture()
	suite.proxyNet = proxy.NewProxyNetwork(suite.net, suite.targetNodeID)
	suite.engine = new(mocknetwork.Engine

	net.On("Register", mock.AnythingOfType("network.Channel"), mock.Anything).Return(suite.con, nil)
}

// TestUnicast tests that the Unicast method is translated to a unicast to the target node
// on the underlying network instance.
func (suite *Suite) TestUnicast() {
	channel := network.Channel("test-channel")
	targetID := unittest.IdentifierFixture()
	event := getEvent()

	con, err := suite.proxyNet.Register(channel, suite.engine)
	suite.Assert().NoError(err)

	suite.con.On("Unicast", event, suite.targetNodeID).Return(nil).Once()

	err = con.Unicast(event, targetID)
	suite.Assert().NoError(err)

	suite.con.AssertNumberOfCalls(suite.T(), "Unicast", 1)
	suite.con.AssertExpectations(suite.T())
}

// TestPublish tests that the Publish method is translated to a publish to the target node
// on the underlying network instance.
func (suite *Suite) TestPublish() {
	channel := network.Channel("test-channel")
	targetIDs := make([]flow.Identifier, 10)

	for i := 0; i < 10; i++ {
		targetIDs = append(targetIDs, unittest.IdentifierFixture())
	}

	event := getEvent()

	con, err := suite.proxyNet.Register(channel, suite.engine)
	suite.Assert().NoError(err)

	suite.con.On("Publish", event, suite.targetNodeID).Return(nil).Once()

	err = con.Publish(event, targetIDs...)
	suite.Assert().NoError(err)

	suite.con.AssertNumberOfCalls(suite.T(), "Publish", 1)
	suite.con.AssertExpectations(suite.T())
}

// TestUnicast tests that the Multicast method is translated to a multicast to the target node
// on the underlying network instance.
func (suite *Suite) TestMulticast() {
	channel := network.Channel("test-channel")
	targetIDs := make([]flow.Identifier, 10)

	for i := 0; i < 10; i++ {
		targetIDs = append(targetIDs, unittest.IdentifierFixture())
	}

	event := getEvent()

	con, err := suite.proxyNet.Register(channel, suite.engine)
	suite.Assert().NoError(err)

	suite.con.On("Multicast", event, uint(1), suite.targetNodeID).Return(nil).Once()

	err = con.Multicast(event, 5, targetIDs...)
	suite.Assert().NoError(err)

	suite.con.AssertNumberOfCalls(suite.T(), "Multicast", 1)
	suite.con.AssertExpectations(suite.T())
}

// TestClose tests that closing the proxy conduit closes the wrapped conduit.
func (suite *Suite) TestClose() {
	channel := network.Channel("test-channel")

	con, err := suite.proxyNet.Register(channel, suite.engine)
	suite.Assert().NoError(err)

	suite.con.On("Close").Return(nil).Once()

	err = con.Close()
	suite.Assert().NoError(err)

	suite.con.AssertNumberOfCalls(suite.T(), "Close", 1)
	suite.con.AssertExpectations(suite.T())
}

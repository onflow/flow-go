package relay

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	engine   *Engine
	channels channels.ChannelList
	conduits map[channels.Channel]*mocknetwork.Conduit
}

func (suite *Suite) SetupTest() {
	suite.channels = channels.ChannelList{
		channels.Channel("test-channel-1"),
	}
	net := new(mocknetwork.Network)
	unstakedNet := new(mocknetwork.Network)
	suite.conduits = make(map[channels.Channel]*mocknetwork.Conduit)

	for _, channel := range suite.channels {
		con := new(mocknetwork.Conduit)
		suite.conduits[channel] = con
		net.On("Register", channel, mock.Anything).Return(new(mocknetwork.Conduit), nil).Once()
		unstakedNet.On("Register", channel, mock.Anything).Return(con, nil).Once()
	}

	eng, err := New(
		zerolog.Logger{},
		suite.channels,
		net,
		unstakedNet,
	)
	suite.Require().Nil(err)

	suite.engine = eng
}

func TestRelayEngine(t *testing.T) {
	suite.Run(t, new(Suite))
}

func getEvent() interface{} {
	return struct {
		foo string
	}{
		foo: "bar",
	}
}

// TestHappyPath tests that the relay engine relays events for each
// channel that it was created with
func (suite *Suite) TestHappyPath() {
	for channel, conduit := range suite.conduits {
		id := unittest.IdentifierFixture()
		event := getEvent()

		conduit.On("Publish", event, flow.ZeroID).Return(nil).Once()

		err := suite.engine.Process(channel, id, event)
		suite.Assert().Nil(err)

		conduit.AssertNumberOfCalls(suite.T(), "Publish", 1)
		conduit.AssertExpectations(suite.T())
	}
}

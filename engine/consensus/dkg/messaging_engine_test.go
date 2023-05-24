package dkg

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	msg "github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/dkg"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

// MessagingEngineSuite encapsulates unit tests for the MessagingEngine.
type MessagingEngineSuite struct {
	suite.Suite

	conduit *mocknetwork.Conduit
	network *mocknetwork.Network
	me      *mockmodule.Local

	engine *MessagingEngine
}

func TestMessagingEngine(t *testing.T) {
	suite.Run(t, new(MessagingEngineSuite))
}

func (ms *MessagingEngineSuite) SetupTest() {
	// setup mock conduit
	ms.conduit = mocknetwork.NewConduit(ms.T())
	ms.network = mocknetwork.NewNetwork(ms.T())
	ms.network.On("Register", mock.Anything, mock.Anything).
		Return(ms.conduit, nil).
		Once()

	// setup local with nodeID
	nodeID := unittest.IdentifierFixture()
	ms.me = mockmodule.NewLocal(ms.T())
	ms.me.On("NodeID").Return(nodeID).Maybe()

	engine, err := NewMessagingEngine(
		unittest.Logger(),
		ms.network,
		ms.me,
		dkg.NewBrokerTunnel(),
		metrics.NewNoopCollector(),
		DefaultMessagingEngineConfig(),
	)
	require.NoError(ms.T(), err)
	ms.engine = engine
}

// TestForwardOutgoingMessages checks that the engine correctly forwards
// outgoing messages from the tunnel's Out channel to the network conduit.
func (ms *MessagingEngineSuite) TestForwardOutgoingMessages() {
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(ms.T(), context.Background())
	ms.engine.Start(ctx)
	defer cancel()

	// expected DKGMessage
	destinationID := unittest.IdentifierFixture()
	expectedMsg := msg.NewDKGMessage(
		[]byte("hello"),
		"dkg-123",
	)

	done := make(chan struct{})
	ms.conduit.On("Unicast", &expectedMsg, destinationID).
		Run(func(_ mock.Arguments) { close(done) }).
		Return(nil).
		Once()

	ms.engine.tunnel.SendOut(msg.PrivDKGMessageOut{
		DKGMessage: expectedMsg,
		DestID:     destinationID,
	})

	unittest.RequireCloseBefore(ms.T(), done, time.Second, "message not sent")
}

// TestForwardIncomingMessages checks that the engine correctly forwards
// messages from the conduit to the tunnel's MsgChIn channel.
func (ms *MessagingEngineSuite) TestForwardIncomingMessages() {
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(ms.T(), context.Background())
	ms.engine.Start(ctx)
	defer cancel()

	originID := unittest.IdentifierFixture()
	expectedMsg := msg.PrivDKGMessageIn{
		DKGMessage: msg.NewDKGMessage([]byte("hello"), "dkg-123"),
		OriginID:   originID,
	}

	// launch a background routine to capture messages forwarded to the tunnel's MsgChIn channel
	done := make(chan struct{})
	go func() {
		receivedMsg := <-ms.engine.tunnel.MsgChIn
		require.Equal(ms.T(), expectedMsg, receivedMsg)
		close(done)
	}()

	err := ms.engine.Process(channels.DKGCommittee, originID, &expectedMsg.DKGMessage)
	require.NoError(ms.T(), err)

	unittest.RequireCloseBefore(ms.T(), done, time.Second, "message not received")
}

package dkg

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	msg "github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/dkg"
	"github.com/onflow/flow-go/module/irrecoverable"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

// Helper function to initialise an engine.
func createTestEngine(t *testing.T) *MessagingEngine {
	// setup mock conduit
	conduit := mocknetwork.NewConduit(t)
	network := mocknetwork.NewNetwork(t)
	network.On("Register", mock.Anything, mock.Anything).
		Return(conduit, nil).
		Once()

	// setup local with nodeID
	nodeID := unittest.IdentifierFixture()
	me := module.NewLocal(t)
	me.On("NodeID").Return(nodeID).Maybe()

	engine, err := NewMessagingEngine(
		unittest.Logger(),
		network,
		me,
		dkg.NewBrokerTunnel(),
	)
	require.NoError(t, err)

	return engine
}

// TestForwardOutgoingMessages checks that the engine correctly forwards
// outgoing messages from the tunnel's Out channel to the network conduit.
func TestForwardOutgoingMessages(t *testing.T) {
	engine := createTestEngine(t)
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())
	engine.Start(ctx)
	defer cancel()

	// expected DKGMessage
	destinationID := unittest.IdentifierFixture()
	expectedMsg := msg.NewDKGMessage(
		[]byte("hello"),
		"dkg-123",
	)

	done := make(chan struct{})
	engine.conduit.(*mocknetwork.Conduit).On("Unicast", &expectedMsg, destinationID).
		Run(func(_ mock.Arguments) { close(done) }).
		Return(nil).
		Once()

	engine.tunnel.SendOut(msg.PrivDKGMessageOut{
		DKGMessage: expectedMsg,
		DestID:     destinationID,
	})

	unittest.RequireCloseBefore(t, done, time.Second, "message not sent")
}

// TestForwardIncomingMessages checks that the engine correctly forwards
// messages from the conduit to the tunnel's In channel.
func TestForwardIncomingMessages(t *testing.T) {
	engine := createTestEngine(t)
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())
	engine.Start(ctx)
	defer cancel()

	originID := unittest.IdentifierFixture()
	expectedMsg := msg.PrivDKGMessageIn{
		DKGMessage: msg.NewDKGMessage([]byte("hello"), "dkg-123"),
		OriginID:   originID,
	}

	// launch a background routine to capture messages forwarded to the tunnel's
	// In channel
	doneCh := make(chan struct{})
	go func() {
		receivedMsg := <-engine.tunnel.MsgChIn
		require.Equal(t, expectedMsg, receivedMsg)
		close(doneCh)
	}()

	err := engine.Process(channels.DKGCommittee, originID, &expectedMsg.DKGMessage)
	require.NoError(t, err)

	unittest.RequireCloseBefore(t, doneCh, time.Second, "message not received")
}

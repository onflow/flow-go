package dkg

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	msg "github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/dkg"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

// Helper function to initialise an engine.
func createTestEngine(t *testing.T) *MessagingEngine {
	// setup mock conduit
	conduit := &mocknetwork.Conduit{}
	network := new(mocknetwork.Network)
	network.On("Register", mock.Anything, mock.Anything).
		Return(conduit, nil).
		Once()

	// setup local with nodeID
	nodeID := unittest.IdentifierFixture()
	me := new(module.Local)
	me.On("NodeID").Return(nodeID)

	engine, err := NewMessagingEngine(
		zerolog.Logger{},
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
	// sender engine
	engine := createTestEngine(t)

	// expected DKGMessage
	destinationID := unittest.IdentifierFixture()
	expectedMsg := msg.NewDKGMessage(
		1,
		[]byte("hello"),
		"dkg-123",
	)

	// override the conduit to check that the Unicast call matches the expected
	// message and destination ID
	conduit := &mocknetwork.Conduit{}
	conduit.On("Unicast", &expectedMsg, destinationID).
		Return(nil).
		Once()
	engine.conduit = conduit

	engine.tunnel.SendOut(msg.PrivDKGMessageOut{
		DKGMessage: expectedMsg,
		DestID:     destinationID,
	})

	time.Sleep(5 * time.Millisecond)

	conduit.AssertExpectations(t)
}

// TestForwardIncomingMessages checks that the engine correclty forwards
// messages from the conduit to the tunnel's In channel.
func TestForwardIncomingMessages(t *testing.T) {
	// sender engine
	e := createTestEngine(t)

	originID := unittest.IdentifierFixture()
	expectedMsg := msg.PrivDKGMessageIn{
		DKGMessage: msg.NewDKGMessage(1, []byte("hello"), "dkg-123"),
		OriginID:   originID,
	}

	// launch a background routine to capture messages forwarded to the tunnel's
	// In channel
	doneCh := make(chan struct{})
	go func() {
		receivedMsg := <-e.tunnel.MsgChIn
		require.Equal(t, expectedMsg, receivedMsg)
		close(doneCh)
	}()

	err := e.Process(engine.DKGCommittee, originID, &expectedMsg.DKGMessage)
	require.NoError(t, err)

	unittest.RequireCloseBefore(t, doneCh, time.Second, "message not received")
}

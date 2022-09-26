package corrupt

import (
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/insecure"
	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestHandleIncomingEvent_AttackerRegistered checks that a corrupt network sends ingress messages to a registered attacker.
// The attacker is mocked out in this test.
func TestHandleIncomingEvent_AttackerRegistered(t *testing.T) {
	codec := unittest.NetworkCodec()
	corruptedIdentity := unittest.IdentityFixture(unittest.WithAddress(insecure.DefaultAddress))
	flowNetwork := &mocknetwork.Network{}
	ccf := &mockinsecure.CorruptConduitFactory{}
	ccf.On("RegisterEgressController", mock.Anything).Return(nil)

	corruptNetwork, err := NewCorruptNetwork(
		unittest.Logger(),
		flow.BftTestnet,
		insecure.DefaultAddress,
		testutil.LocalFixture(t, corruptedIdentity),
		codec,
		flowNetwork,
		ccf)
	require.NoError(t, err)

	attacker := newMockAttacker()

	attackerRegistered := sync.WaitGroup{}
	attackerRegistered.Add(1)
	go func() {
		attackerRegistered.Done()

		err := corruptNetwork.ConnectAttacker(&empty.Empty{}, attacker) // blocking call
		require.NoError(t, err)
	}()
	unittest.RequireReturnsBefore(t, attackerRegistered.Wait, 1*time.Second, "could not register attacker on time")

	originId := unittest.IdentifierFixture()
	msg := &message.TestMessage{Text: "this is a test msg"}
	channel := channels.TestNetworkChannel

	go func() {
		isAttackerRegistered := corruptNetwork.HandleIncomingEvent(msg, channel, originId)
		require.True(t, isAttackerRegistered, "attacker should be registered")
	}()

	// For this test we use a mock attacker, that puts the incoming messages into a channel. Then in this test we keep reading from that channel till
	// either a message arrives or a timeout. Reading a message from that channel means attackers Observe has been called.
	var receivedMsg *insecure.Message
	unittest.RequireReturnsBefore(t, func() {
		receivedMsg = <-attacker.incomingBuffer
	}, 100*time.Millisecond, "mock attack could not receive incoming message on time")

	// checks content of the received message matches what has been sent.
	receivedId, err := flow.ByteSliceToId(receivedMsg.Ingress.OriginID)
	require.NoError(t, err)
	require.Equal(t, originId, receivedId)
	require.Equal(t, receivedMsg.Ingress.ChannelID, string(channel))

	decodedEvent, err := codec.Decode(receivedMsg.Ingress.Payload)
	require.NoError(t, err)
	require.Equal(t, msg, decodedEvent)
	mock.AssertExpectationsForObjects(t, ccf)
}

// TestHandleIncomingEvent_NoAttacker checks that incoming events to the corrupted network
// are routed to the network adapter when no attacker is registered to the network.
func TestHandleIncomingEvent_NoAttacker(t *testing.T) {
	corruptNetwork, adapter := corruptNetworkFixture(t, unittest.Logger())

	originId := unittest.IdentifierFixture()
	msg := &message.TestMessage{Text: "this is a test msg"}
	channel := channels.TestNetworkChannel

	// simulate sending message by conduit
	isAttackerRegistered := corruptNetwork.HandleIncomingEvent(msg, channel, originId)
	require.False(t, isAttackerRegistered, "attacker should not be registered")

	// check that correct Adapter method called
	mock.AssertExpectationsForObjects(t, adapter)
}

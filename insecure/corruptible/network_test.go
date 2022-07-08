package corruptible

import (
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/insecure"
	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

// TestFactoryHandleIncomingEvent_AttackerObserve evaluates that the incoming messages to the corrupted network are routed to the
// registered attacker if one exists.
func TestFactoryHandleIncomingEvent_AttackerObserve(t *testing.T) {
	codec := cbor.NewCodec()

	corruptedIdentity := unittest.IdentityFixture(unittest.WithAddress("localhost:0"))

	//ccf := NewCorruptibleConduitFactory(unittest.Logger(), flow.BftTestnet)
	flowNetwork := &mocknetwork.Network{}

	//ccfMock := &mocknetwork.ConduitFactory{}

	ccfMock2 := &mockinsecure.CorruptibleConduitFactory{}

	corruptibleNetwork, err := NewCorruptibleNetwork(
		unittest.Logger(),
		flow.BftTestnet,
		"localhost:0",
		testutil.LocalFixture(t, corruptedIdentity),
		codec,
		flowNetwork,
		ccfMock2)
	require.NoError(t, err)

	attacker := newMockAttackerObserverClient()

	attackerRegistered := sync.WaitGroup{}
	attackerRegistered.Add(1)
	go func() {
		attackerRegistered.Done()

		err := corruptibleNetwork.ConnectAttacker(&empty.Empty{}, attacker) // blocking call
		require.NoError(t, err)
	}()
	unittest.RequireReturnsBefore(t, attackerRegistered.Wait, 1*time.Second, "could not register attacker on time")

	event := &message.TestMessage{Text: "this is a test message"}
	targetIds := unittest.IdentifierListFixture(10)
	channel := network.Channel("test-channel")

	go func() {
		err := corruptibleNetwork.HandleOutgoingEvent(event, channel, insecure.Protocol_MULTICAST, uint32(3), targetIds...)
		require.NoError(t, err)
	}()

	// For this test we use a mock attacker, that puts the incoming messages into a channel. Then in this test we keep reading from that channel till
	// either a message arrives or a timeout. Reading a message from that channel means attackers Observe has been called.
	var receivedMsg *insecure.Message
	unittest.RequireReturnsBefore(t, func() {
		receivedMsg = <-attacker.incomingBuffer
	}, 100*time.Millisecond, "mock attack could not receive incoming message on time")

	// checks content of the received message matches what has been sent.
	require.ElementsMatch(t, receivedMsg.TargetIDs, flow.IdsToBytes(targetIds))
	require.Equal(t, receivedMsg.TargetNum, uint32(3))
	require.Equal(t, receivedMsg.Protocol, insecure.Protocol_MULTICAST)
	require.Equal(t, receivedMsg.ChannelID, string(channel))

	decodedEvent, err := codec.Decode(receivedMsg.Payload)
	require.NoError(t, err)
	require.Equal(t, event, decodedEvent)
}

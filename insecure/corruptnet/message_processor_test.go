package corruptnet

import (
	"testing"

	"github.com/onflow/flow-go/network/mocknetwork"

	"github.com/stretchr/testify/require"

	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestProcess_AttackerRegistered checks that when an attacker is registered with a corrupt network, the corrupt
// message processor does NOT send the message to the original message processor.
// This test simulates the attacker being registered by making HandleIncomingEvent() return true.
// This test will fail if originalProcessor is called.
func TestProcess_AttackerRegistered(t *testing.T) {
	originalProcessor := mocknetwork.NewEngine(t)

	channel := channels.TestNetworkChannel
	originId := unittest.IdentifierFixture()
	msg := &message.TestMessage{Text: "this is a test msg"}

	ingressController := mockinsecure.NewIngressController(t)
	ingressController.On("HandleIncomingEvent", msg, channel, originId).Return(true)
	messageProcessor := NewCorruptMessageProcessor(unittest.Logger(), originalProcessor, ingressController)

	err := messageProcessor.Process(channel, originId, msg)
	require.NoError(t, err)
}

// TestProcess_AttackerRegistered checks that when an attacker is NOT registered with a corrupt network, the
// corrupt message processor sends the incoming message on the original message processor.
// This test simulates the attacker not being registered by making HandleIncomingEvent() return false.
func TestProcess_AttackerNotRegistered(t *testing.T) {
	originalProcessor := mocknetwork.NewEngine(t)

	channel := channels.TestNetworkChannel
	originId := unittest.IdentifierFixture()
	msg := &message.TestMessage{Text: "this is a test msg"}

	ingressController := mockinsecure.NewIngressController(t)
	ingressController.On("HandleIncomingEvent", msg, channel, originId).Return(false)

	corruptChannel := channels.TestNetworkChannel
	ingressMsg := &message.TestMessage{Text: "this is a test msg"}

	// this simulates the corrupt message processor sending the message on the original message processor when an attacker is not registered
	originalProcessor.On("Process", corruptChannel, originId, ingressMsg).Return(nil)

	messageProcessor := NewCorruptMessageProcessor(unittest.Logger(), originalProcessor, ingressController)

	err := messageProcessor.Process(corruptChannel, originId, ingressMsg)
	require.NoError(t, err)
}

package net

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/network"
)

// TestProcess_AttackerRegistered checks that when an attacker is registered with a corrupt network, the corrupt
// message processor does NOT send the message to the original message processor.
// This test simulates the attacker not being registered by making HandleIncomingEvent() return true.
func TestProcess_AttackerRegistered(t *testing.T) {
	originalProcessor := &network.Engine{}
	ingressController := mockinsecure.NewIngressController(t)
	ingressController.On("HandleIncomingEvent", mock.Anything, mock.Anything, mock.Anything).Return(true)
	messageProcessor := NewCorruptMessageProcessor(unittest.Logger(), originalProcessor, ingressController)

	channel := channels.TestNetworkChannel
	originId := unittest.IdentifierFixture()
	msg := &message.TestMessage{Text: "this is a test msg"}

	err := messageProcessor.Process(channel, originId, msg)
	require.NoError(t, err)
}

// TestProcess_AttackerRegistered checks that when an attacker is NOT registered with a corrupt network, the
// corrupt message processor sends the incoming message on the original message processor.
// This test simulates the attacker not being registered by making HandleIncomingEvent() return false.
func TestProcess_AttackerNotRegistered(t *testing.T) {
	originalProcessor := &network.Engine{}
	ingressController := mockinsecure.NewIngressController(t)
	ingressController.On("HandleIncomingEvent", mock.Anything, mock.Anything, mock.Anything).Return(false)
	originId := unittest.IdentifierFixture()
	corruptChannel := channels.TestNetworkChannel
	corruptMsg := &message.TestMessage{Text: "this is a test msg"}

	// this simulates the corrupt message processor sending the message on the original message processor when an attacker is not registered
	originalProcessor.OnProcess(func(originalChannel channels.Channel, originalOriginId flow.Identifier, originalMessage interface{}) error {
		// check that none of the data has been altered when sending on the original message processor
		require.Equal(t, corruptChannel, originalChannel)
		require.Equal(t, originId, originalOriginId)
		require.Equal(t, corruptMsg, originalMessage)
		return nil
	})

	messageProcessor := NewCorruptMessageProcessor(unittest.Logger(), originalProcessor, ingressController)

	err := messageProcessor.Process(corruptChannel, originId, corruptMsg)
	require.NoError(t, err)
}

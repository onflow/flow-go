package messageutils

import (
	"testing"

	"github.com/onflow/flow-go/network"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	libp2pmessage "github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/utils/unittest"
)

func CreateMessage(t *testing.T, originID flow.Identifier, targetID flow.Identifier, channel channels.Channel, msg string) (*message.Message, *message.Message, interface{}) {
	payload := &libp2pmessage.TestMessage{
		Text: msg,
	}

	return CreateMessageWithPayload(t, originID, targetID, channel, payload)
}

func CreateMessageWithPayload(t *testing.T, originID flow.Identifier, targetID flow.Identifier, channel channels.Channel, payload interface{}) (*message.Message, *message.Message, interface{}) {
	codec := unittest.NetworkCodec()
	b, err := codec.Encode(payload)
	require.NoError(t, err)

	m := &message.Message{
		ChannelID: channel.String(),
		TargetIDs: [][]byte{targetID[:]},
		Payload:   b,
	}

	eventID, err := network.EventId(channel, m.Payload)
	require.NoError(t, err)

	// this is the message after all network processing. i.e. what is passed to network.Receive
	outputMsg := &message.Message{
		ChannelID: m.ChannelID,
		TargetIDs: m.TargetIDs,
		Payload:   m.Payload,
		EventID:   eventID,
		OriginID:  originID[:],
		Type:      network.MessageType(payload),
	}

	return m, outputMsg, payload
}

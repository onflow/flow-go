package messageutils

import (
	"testing"

	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	libp2pmessage "github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/utils/unittest"
)

func CreateMessage(t *testing.T, originID flow.Identifier, targetID flow.Identifier, channel channels.Channel, msg string) (*message.Message, interface{}) {
	payload := &libp2pmessage.TestMessage{
		Text: msg,
	}

	codec := unittest.NetworkCodec()
	b, err := codec.Encode(payload)
	require.NoError(t, err)

	eventID, err := p2p.EventId(channel, b)
	require.NoError(t, err)

	m := &message.Message{
		ChannelID: channel.String(),
		EventID:   eventID,
		OriginID:  originID[:],
		TargetIDs: [][]byte{targetID[:]},
		Payload:   b,
		Type:      p2p.MessageType(payload),
	}

	return m, payload
}

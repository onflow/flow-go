package messageutils

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	libp2pmessage "github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/utils/unittest"
)

func CreateMessage(originID flow.Identifier, targetID flow.Identifier, channel, msg string) (*message.Message, interface{}) {
	payload := &libp2pmessage.TestMessage{
		Text: msg,
	}

	codec := unittest.NetworkCodec()
	b, err := codec.Encode(payload)
	if err != nil {
		fmt.Println(err)
	}

	m := &message.Message{
		ChannelID: channel,
		Type:      flow.MakeID(payload).String(),
		EventID:   []byte("1"),
		OriginID:  originID[:],
		TargetIDs: [][]byte{targetID[:]},
		Payload:   b,
	}

	return m, payload
}

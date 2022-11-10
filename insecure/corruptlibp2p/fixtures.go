package corruptlibp2p

import (
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"

	"github.com/onflow/flow-go/utils/unittest"
)

type GossipSubCtrlOption func(*pubsubpb.ControlMessage)

func GossipSubCtrlFixture(opts ...GossipSubCtrlOption) *pubsubpb.ControlMessage {
	msg := &pubsubpb.ControlMessage{}
	for _, opt := range opts {
		opt(msg)
	}
	return msg
}

func WithIHave(msgCount int, msgSize int) GossipSubCtrlOption {
	return func(msg *pubsubpb.ControlMessage) {
		iHaves := make([]*pubsubpb.ControlIHave, msgCount)
		for i := 0; i < msgCount; i++ {
			topicId := topicIdFixture()
			iHaves[i] = &pubsubpb.ControlIHave{
				TopicID:    &topicId,
				MessageIDs: messageIdsFixture(msgSize),
			}
		}
		msg.Ihave = iHaves
	}
}

func messageIdFixture() string {
	// TODO: messageID length should be a parameter.
	return unittest.GenerateRandomStringWithLen(10)
}

func topicIdFixture() string {
	// TODO: topicID length should be a parameter.
	return unittest.GenerateRandomStringWithLen(10)
}

func messageIdsFixture(count int) []string {
	msgIds := make([]string, count)
	for i := 0; i < count; i++ {
		msgIds[i] = messageIdFixture()
	}
	return msgIds
}

package corruptlibp2p

import (
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"

	"github.com/onflow/flow-go/utils/unittest"
)

type GossipSubCtrlOption func(*pubsubpb.ControlMessage)

// GossipSubCtrlFixture returns a ControlMessage with the given options.
func GossipSubCtrlFixture(opts ...GossipSubCtrlOption) *pubsubpb.ControlMessage {
	msg := &pubsubpb.ControlMessage{}
	for _, opt := range opts {
		opt(msg)
	}
	return msg
}

// WithIHave adds iHave control messages of the given size and number to the control message.
func WithIHave(msgCount int, msgSize int) GossipSubCtrlOption {
	return func(msg *pubsubpb.ControlMessage) {
		iHaves := make([]*pubsubpb.ControlIHave, msgCount)
		for i := 0; i < msgCount; i++ {
			topicId := gossipSubTopicIdFixture()
			iHaves[i] = &pubsubpb.ControlIHave{
				TopicID:    &topicId,
				MessageIDs: gossipSubMessageIdsFixture(msgSize),
			}
		}
		msg.Ihave = iHaves
	}
}

// gossipSubMessageIdFixture returns a random gossipSub message ID.
func gossipSubMessageIdFixture() string {
	// TODO: messageID length should be a parameter.
	return unittest.GenerateRandomStringWithLen(10)
}

// gossipSubTopicIdFixture returns a random gossipSub topic ID.
func gossipSubTopicIdFixture() string {
	// TODO: topicID length should be a parameter.
	return unittest.GenerateRandomStringWithLen(10)
}

// gossipSubMessageIdsFixture returns a slice of random gossipSub message IDs of the given size.
func gossipSubMessageIdsFixture(count int) []string {
	msgIds := make([]string, count)
	for i := 0; i < count; i++ {
		msgIds[i] = gossipSubMessageIdFixture()
	}
	return msgIds
}

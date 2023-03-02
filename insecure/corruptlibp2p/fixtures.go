package corruptlibp2p

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	// topicIDFixtureLen is the length of the topic ID fixture for testing.
	topicIDFixtureLen = 10
	// messageIDFixtureLen is the length of the message ID fixture for testing.
	messageIDFixtureLen = 10
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
	return unittest.GenerateRandomStringWithLen(messageIDFixtureLen)
}

// gossipSubTopicIdFixture returns a random gossipSub topic ID.
func gossipSubTopicIdFixture() string {
	// TODO: topicID length should be a parameter.
	return unittest.GenerateRandomStringWithLen(topicIDFixtureLen)
}

// gossipSubMessageIdsFixture returns a slice of random gossipSub message IDs of the given size.
func gossipSubMessageIdsFixture(count int) []string {
	msgIds := make([]string, count)
	for i := 0; i < count; i++ {
		msgIds[i] = gossipSubMessageIdFixture()
	}
	return msgIds
}

func CorruptInspectorFunc(inspector p2p.GossipSubRPCInspector) func(id peer.ID, rpc *corrupt.RPC) error {
	return func(id peer.ID, rpc *corrupt.RPC) error {
		pubsubrpc := &pubsub.RPC{
			RPC: pubsubpb.RPC{
				Subscriptions:        rpc.Subscriptions,
				Publish:              rpc.Publish,
				Control:              rpc.Control,
				XXX_NoUnkeyedLiteral: rpc.XXX_NoUnkeyedLiteral,
				XXX_unrecognized:     rpc.XXX_unrecognized,
				XXX_sizecache:        rpc.XXX_sizecache,
			},
		}
		return inspector.Inspect(id, pubsubrpc)
	}
}

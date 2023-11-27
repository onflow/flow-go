package corruptlibp2p

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/unittest"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"
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
// The message IDs are generated randomly.
// Args:
//
//	msgCount: number of iWant messages to add.
//	msgIDCount: number of message IDs to add to each iWant message.
//
// Returns:
// A GossipSubCtrlOption that adds iWant messages to the control message.
// Example: WithIHave(2, 3, channels.Pushblocks) will add 2 iHave messages, each with 3 message IDs on the push-blocks topic.
func WithIHave(msgCount, msgIDCount int, topicId string) GossipSubCtrlOption {
	return func(msg *pubsubpb.ControlMessage) {
		iHaves := make([]*pubsubpb.ControlIHave, msgCount)
		for i := 0; i < msgCount; i++ {
			iHaves[i] = &pubsubpb.ControlIHave{
				TopicID:    &topicId,
				MessageIDs: GossipSubMessageIdsFixture(msgIDCount),
			}
		}
		msg.Ihave = iHaves
	}
}

// WithIHaves adds an iHave control message for each topic ID.
// The message IDs are generated randomly.
// Args:
//
//	msgIDCount: number of message IDs to add to each iHave message.
//	topicIds: list of topic ids to make an iHave message for.
//
// Returns:
// A GossipSubCtrlOption that adds iHave messages to the control message.
// Example: WithIHaves(3, channels.Pushblocks, channels.TestNetwork, channels.Topic("some channel")) will add an iHave message for each topic, each with 3 message IDs.
func WithIHaves(msgIDCount int, topicIds ...string) GossipSubCtrlOption {
	return func(msg *pubsubpb.ControlMessage) {
		iHaves := make([]*pubsubpb.ControlIHave, len(topicIds))
		for i, topicId := range topicIds {
			topicId := topicId
			iHaves[i] = &pubsubpb.ControlIHave{
				TopicID:    &topicId,
				MessageIDs: GossipSubMessageIdsFixture(msgIDCount),
			}
		}
		msg.Ihave = iHaves
	}
}

// WithIWant adds an iWant control message for with the provided message IDs.
// Args:
//
//	msgIds: list of message ids.
//
// Returns:
// A GossipSubCtrlOption that adds iWant messages to the control message.
// Example: WithIWant("message_id_1", "message_id_2", "message_id_3", "message_id_4) will add an iWant message for each topic, each with 3 message IDs.
func WithIWant(msgIds ...string) GossipSubCtrlOption {
	return func(msg *pubsubpb.ControlMessage) {
		msg.Iwant = []*pubsubpb.ControlIWant{{
			MessageIDs: msgIds,
		}}
	}
}

// WithIWants adds iWant control messages of the given size and number to the control message.
// The message IDs are generated randomly.
// Args:
//
//	msgCount: number of iWant messages to add.
//	msgIdsPerIWant: number of message IDs to add to each iWant message.
//
// Returns:
// A GossipSubCtrlOption that adds iWant messages to the control message.
// Example: WithIWants(2, 3) will add 2 iWant messages, each with 3 message IDs.
func WithIWants(iWantCount int, msgIdsPerIWant int) GossipSubCtrlOption {
	return func(msg *pubsubpb.ControlMessage) {
		iWants := make([]*pubsubpb.ControlIWant, iWantCount)
		for i := 0; i < iWantCount; i++ {
			iWants[i] = &pubsubpb.ControlIWant{
				MessageIDs: GossipSubMessageIdsFixture(msgIdsPerIWant),
			}
		}
		msg.Iwant = iWants
	}
}

// WithGrafts adds GRAFT control messages with each given topicID to the control message.
func WithGrafts(topicIds ...string) GossipSubCtrlOption {
	return func(msg *pubsubpb.ControlMessage) {
		grafts := make([]*pubsubpb.ControlGraft, len(topicIds))
		for i, topicId := range topicIds {
			topicId := topicId
			grafts[i] = &pubsubpb.ControlGraft{
				TopicID: &topicId,
			}
		}
		msg.Graft = grafts
	}
}

// WithGraft adds GRAFT control messages with given topicID to the control message.
func WithGraft(msgCount int, topicId string) GossipSubCtrlOption {
	return func(msg *pubsubpb.ControlMessage) {
		grafts := make([]*pubsubpb.ControlGraft, msgCount)
		for i := 0; i < msgCount; i++ {
			grafts[i] = &pubsubpb.ControlGraft{
				TopicID: &topicId,
			}
		}
		msg.Graft = grafts
	}
}

// WithPrunes adds PRUNE control messages with each given topicID to the control message.
func WithPrunes(topicIds ...string) GossipSubCtrlOption {
	return func(msg *pubsubpb.ControlMessage) {
		prunes := make([]*pubsubpb.ControlPrune, len(topicIds))
		for i, topicId := range topicIds {
			topicId := topicId
			prunes[i] = &pubsubpb.ControlPrune{
				TopicID: &topicId,
			}
		}
		msg.Prune = prunes
	}
}

// WithPrune adds PRUNE control messages with given topicID to the control message.
func WithPrune(msgCount int, topicId string) GossipSubCtrlOption {
	return func(msg *pubsubpb.ControlMessage) {
		prunes := make([]*pubsubpb.ControlPrune, msgCount)
		for i := 0; i < msgCount; i++ {
			prunes[i] = &pubsubpb.ControlPrune{
				TopicID: &topicId,
			}
		}
		msg.Prune = prunes
	}
}

// gossipSubMessageIdFixture returns a random gossipSub message ID.
func gossipSubMessageIdFixture() string {
	// TODO: messageID length should be a parameter.
	return unittest.GenerateRandomStringWithLen(messageIDFixtureLen)
}

// GossipSubTopicIdFixture returns a random gossipSub topic ID.
func GossipSubTopicIdFixture() string {
	// TODO: topicID length should be a parameter.
	return unittest.GenerateRandomStringWithLen(topicIDFixtureLen)
}

// GossipSubMessageIdsFixture returns a slice of random gossipSub message IDs of the given size.
func GossipSubMessageIdsFixture(count int) []string {
	msgIds := make([]string, count)
	for i := 0; i < count; i++ {
		msgIds[i] = gossipSubMessageIdFixture()
	}
	return msgIds
}

// CorruptInspectorFunc wraps a normal RPC inspector with a corrupt inspector func by translating corrupt.RPC -> pubsubpb.RPC
// before calling Inspect func.
func CorruptInspectorFunc(inspector p2p.GossipSubRPCInspector) func(id peer.ID, rpc *corrupt.RPC) error {
	return func(id peer.ID, rpc *corrupt.RPC) error {
		return inspector.Inspect(id, CorruptRPCToPubSubRPC(rpc))
	}
}

// CorruptRPCToPubSubRPC translates a corrupt.RPC -> pubsub.RPC
func CorruptRPCToPubSubRPC(rpc *corrupt.RPC) *pubsub.RPC {
	return &pubsub.RPC{
		RPC: pubsubpb.RPC{
			Subscriptions:        rpc.Subscriptions,
			Publish:              rpc.Publish,
			Control:              rpc.Control,
			XXX_NoUnkeyedLiteral: rpc.XXX_NoUnkeyedLiteral,
			XXX_unrecognized:     rpc.XXX_unrecognized,
			XXX_sizecache:        rpc.XXX_sizecache,
		},
	}
}

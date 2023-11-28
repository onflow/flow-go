package p2p

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/module/component"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

// GossipSubInspectorNotifDistributor is the interface for the distributor that distributes gossip sub inspector notifications.
// It is used to distribute notifications to the consumers in an asynchronous manner and non-blocking manner.
// The implementation should guarantee that all registered consumers are called upon distribution of a new event.
type GossipSubInspectorNotifDistributor interface {
	component.Component
	// Distribute distributes the event to all the consumers.
	// Any error returned by the distributor is non-recoverable and will cause the node to crash.
	// Implementation must be concurrency safe, and non-blocking.
	Distribute(notification *InvCtrlMsgNotif) error

	// AddConsumer adds a consumer to the distributor. The consumer will be called the distributor distributes a new event.
	// AddConsumer must be concurrency safe. Once a consumer is added, it must be called for all future events.
	// There is no guarantee that the consumer will be called for events that were already received by the distributor.
	AddConsumer(GossipSubInvCtrlMsgNotifConsumer)
}

// CtrlMsgTopicType represents the type of the topic within a control message.
type CtrlMsgTopicType int

const (
	// CtrlMsgNonClusterTopicType represents a non-cluster-prefixed topic.
	CtrlMsgNonClusterTopicType CtrlMsgTopicType = iota
	// CtrlMsgTopicTypeClusterPrefixed represents a cluster-prefixed topic.
	CtrlMsgTopicTypeClusterPrefixed
)

func (t CtrlMsgTopicType) String() string {
	switch t {
	case CtrlMsgNonClusterTopicType:
		return "non-cluster-prefixed"
	case CtrlMsgTopicTypeClusterPrefixed:
		return "cluster-prefixed"
	default:
		return "unknown"
	}
}

// InvCtrlMsgNotif is the notification sent to the consumer when an invalid control message is received.
// It models the information that is available to the consumer about a misbehaving peer.
type InvCtrlMsgNotif struct {
	// PeerID is the ID of the peer that sent the invalid control message.
	PeerID peer.ID
	// Error the error that occurred during validation.
	Error error
	// MsgType the control message type.
	MsgType p2pmsg.ControlMessageType
	// Count the number of errors.
	Count uint64
	// TopicType reports whether the error occurred on a cluster-prefixed topic within the control message.
	// Notifications must be explicitly marked as cluster-prefixed or not because the penalty applied to the GossipSub score
	// for an error on a cluster-prefixed topic is more lenient than the penalty applied to a non-cluster-prefixed topic.
	// This distinction ensures that nodes engaged in cluster-prefixed topic communication are not penalized too harshly,
	// as such communication is vital to the progress of the chain.
	TopicType CtrlMsgTopicType
}

// NewInvalidControlMessageNotification returns a new *InvCtrlMsgNotif
// Args:
//   - peerID: peer id of the offender.
//   - ctlMsgType: the control message type of the rpc message that caused the error.
//   - err: the error that occurred.
//   - count: the number of occurrences of the error.
//
// Returns:
//   - *InvCtlMsgNotif: invalid control message notification.
func NewInvalidControlMessageNotification(peerID peer.ID, ctlMsgType p2pmsg.ControlMessageType, err error, count uint64, topicType CtrlMsgTopicType) *InvCtrlMsgNotif {
	return &InvCtrlMsgNotif{
		PeerID:            peerID,
		Error:             err,
		MsgType:           ctlMsgType,
		Count:             count,
		TopicType: topicType,
	}
}

// GossipSubInvCtrlMsgNotifConsumer is the interface for the consumer that consumes gossipsub inspector notifications.
// It is used to consume notifications in an asynchronous manner.
// The implementation must be concurrency safe, but can be blocking. This is due to the fact that the consumer is called
// asynchronously by the distributor.
type GossipSubInvCtrlMsgNotifConsumer interface {
	// OnInvalidControlMessageNotification is called when a new invalid control message notification is distributed.
	// Any error on consuming event must handle internally.
	// The implementation must be concurrency safe, but can be blocking.
	OnInvalidControlMessageNotification(*InvCtrlMsgNotif)
}

// GossipSubInspectorSuite is the interface for the GossipSub inspector suite.
// It encapsulates the rpc inspectors and the notification distributors.
type GossipSubInspectorSuite interface {
	component.Component
	CollectionClusterChangesConsumer
	// InspectFunc returns the inspect function that is used to inspect the gossipsub rpc messages.
	// This function follows a dependency injection pattern, where the inspect function is injected into the gossipsu, and
	// is called whenever a gossipsub rpc message is received.
	InspectFunc() func(peer.ID, *pubsub.RPC) error

	// AddInvalidControlMessageConsumer adds a consumer to the invalid control message notification distributor.
	// This consumer is notified when a misbehaving peer regarding gossipsub control messages is detected. This follows a pub/sub
	// pattern where the consumer is notified when a new notification is published.
	// A consumer is only notified once for each notification, and only receives notifications that were published after it was added.
	AddInvalidControlMessageConsumer(GossipSubInvCtrlMsgNotifConsumer)

	// SetTopicOracle sets the topic oracle of the gossipsub inspector suite.
	// The topic oracle is used to determine the list of topics that the node is subscribed to.
	// If an oracle is not set, the node will not be able to determine the list of topics that the node is subscribed to.
	// This func is expected to be called once and will return an error on all subsequent calls.
	// All errors returned from this func are considered irrecoverable.
	SetTopicOracle(topicOracle func() []string) error
}

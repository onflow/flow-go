package p2p

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
)

// DisallowListConsumer consumes notifications from the cache.NodeBlocklistWrapper whenever the block list is updated.
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
type DisallowListConsumer interface {
	// OnNodeDisallowListUpdate notifications whenever the node block list is updated.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnNodeDisallowListUpdate(list flow.IdentifierList)
}

// ControlMessageType is the type of control message, as defined in the libp2p pubsub spec.
type ControlMessageType string

const (
	CtrlMsgIHave ControlMessageType = "IHAVE"
	CtrlMsgIWant ControlMessageType = "IWANT"
	CtrlMsgGraft ControlMessageType = "GRAFT"
	CtrlMsgPrune ControlMessageType = "PRUNE"
)

// DisallowListUpdateNotification is the event that is submitted to the distributor when the disallow list is updated.
type DisallowListUpdateNotification struct {
	DisallowList flow.IdentifierList
}

type DisallowListNotificationConsumer interface {
	// OnDisallowListNotification is called when a new disallow list update notification is distributed.
	// Any error on consuming event must handle internally.
	// The implementation must be concurrency safe, but can be blocking.
	OnDisallowListNotification(*DisallowListUpdateNotification)
}

type DisallowListNotificationDistributor interface {
	// DistributeBlockListNotification distributes the event to all the consumers.
	// Any error returned by the distributor is non-recoverable and will cause the node to crash.
	// Implementation must be concurrency safe, and non-blocking.
	DistributeBlockListNotification(list flow.IdentifierList) error

	// AddConsumer adds a consumer to the distributor. The consumer will be called the distributor distributes a new event.
	// AddConsumer must be concurrency safe. Once a consumer is added, it must be called for all future events.
	// There is no guarantee that the consumer will be called for events that were already received by the distributor.
	AddConsumer(DisallowListNotificationConsumer)
}

// GossipSubInspectorNotificationDistributor is the interface for the distributor that distributes gossip sub inspector notifications.
// It is used to distribute notifications to the consumers in an asynchronous manner and non-blocking manner.
// The implementation should guarantee that all registered consumers are called upon distribution of a new event.
type GossipSubInspectorNotificationDistributor interface {
	// DistributeInvalidControlMessageNotification distributes the event to all the consumers.
	// Any error returned by the distributor is non-recoverable and will cause the node to crash.
	// Implementation must be concurrency safe, and non-blocking.
	DistributeInvalidControlMessageNotification(notification *InvalidControlMessageNotification) error

	// AddConsumer adds a consumer to the distributor. The consumer will be called the distributor distributes a new event.
	// AddConsumer must be concurrency safe. Once a consumer is added, it must be called for all future events.
	// There is no guarantee that the consumer will be called for events that were already received by the distributor.
	AddConsumer(GossipSubInvalidControlMessageNotificationConsumer)
}

// InvalidControlMessageNotification is the notification sent to the consumer when an invalid control message is received.
// It models the information that is available to the consumer about a misbehaving peer.
type InvalidControlMessageNotification struct {
	// PeerID is the ID of the peer that sent the invalid control message.
	PeerID peer.ID
	// MsgType is the type of control message that was received.
	MsgType ControlMessageType
	// Count is the number of invalid control messages received from the peer that is reported in this notification.
	Count uint64
}

// GossipSubInvalidControlMessageNotificationConsumer is the interface for the consumer that consumes gossip sub inspector notifications.
// It is used to consume notifications in an asynchronous manner.
// The implementation must be concurrency safe, but can be blocking. This is due to the fact that the consumer is called
// asynchronously by the distributor.
type GossipSubInvalidControlMessageNotificationConsumer interface {
	// OnInvalidControlMessageNotification is called when a new invalid control message notification is distributed.
	// Any error on consuming event must handle internally.
	// The implementation must be concurrency safe, but can be blocking.
	OnInvalidControlMessageNotification(*InvalidControlMessageNotification)
}

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
func NewInvalidControlMessageNotification(peerID peer.ID, ctlMsgType p2pmsg.ControlMessageType, err error, count uint64) *InvCtrlMsgNotif {
	return &InvCtrlMsgNotif{
		PeerID:  peerID,
		Error:   err,
		MsgType: ctlMsgType,
		Count:   count,
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
}

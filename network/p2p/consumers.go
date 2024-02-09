package p2p

import (
	"github.com/libp2p/go-libp2p/core/peer"

	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

// CtrlMsgTopicType represents the type of the topic within a control message.
type CtrlMsgTopicType uint64

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
		PeerID:    peerID,
		Error:     err,
		MsgType:   ctlMsgType,
		Count:     count,
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
	// The implementation must be concurrency safe and non-blocking.
	OnInvalidControlMessageNotification(*InvCtrlMsgNotif)
}

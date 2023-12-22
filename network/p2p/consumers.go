package p2p

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/module/component"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

// InvCtrlMsgErrs list of InvCtrlMsgErr's
type InvCtrlMsgErrs []*InvCtrlMsgErr

func (i InvCtrlMsgErrs) Error() error {
	var errs *multierror.Error
	for _, err := range i {
		switch {
		case err.Topic() != "":
			errs = multierror.Append(errs, fmt.Errorf("%w (topic: %s)", err.Err, err.Topic()))
		case err.MessageID() != "":
			errs = multierror.Append(errs, fmt.Errorf("%w (messageID: %s)", err.Err, err.MessageID()))
		default:
			errs = multierror.Append(errs, err.Err)
		}
	}
	return errs.ErrorOrNil()
}

func (i InvCtrlMsgErrs) Len() int {
	return len(i)
}

// InvCtrlMsgErr represents an error that occurred during control message inspection.
// It encapsulates the error itself along with additional metadata, including the control message type,
// the associated topic or message ID.
type InvCtrlMsgErr struct {
	// Err holds the underlying error.
	Err error

	// ctrlMsgTopicType specifies the type of control message.
	ctrlMsgTopicType CtrlMsgTopicType

	// topic is the topic associated with the error.
	topic string

	// messageID is the message ID associated with an error during iWant validation.
	messageID string
}

// SetTopic sets the topic of the error.
func (e *InvCtrlMsgErr) SetTopic(topic string) {
	e.topic = topic
}

// SetMessageID sets the provided messageID.
func (e *InvCtrlMsgErr) SetMessageID(messageID string) {
	e.messageID = messageID
}

// MessageID returns the messageID of the error.
func (e *InvCtrlMsgErr) MessageID() string {
	return e.messageID
}

// Topic returns the topi of the error.
func (e *InvCtrlMsgErr) Topic() string {
	return e.topic
}

// CtrlMsgTopicType returns the CtrlMsgTopicType of the error.
func (e *InvCtrlMsgErr) CtrlMsgTopicType() CtrlMsgTopicType {
	return e.ctrlMsgTopicType
}

// NewInvCtrlMsgErr returns a new InvCtrlMsgErr.
// Args:
// - err: the error.
// - ctrlMsgTopicType: the control message topic type.
// Returns:
// - *InvCtrlMsgErr: the invalid control message error.
func NewInvCtrlMsgErr(err error, ctrlMsgTopicType CtrlMsgTopicType) *InvCtrlMsgErr {
	return &InvCtrlMsgErr{
		Err:              err,
		ctrlMsgTopicType: ctrlMsgTopicType,
	}
}

// InvCtrlMsgNotif is the notification sent to the consumer when an invalid control message is received.
// It models the information that is available to the consumer about a misbehaving peer.
type InvCtrlMsgNotif struct {
	// PeerID is the ID of the peer that sent the invalid control message.
	PeerID peer.ID
	// Errors the errors that occurred during validation.
	Errors InvCtrlMsgErrs
	// MsgType the control message type.
	MsgType p2pmsg.ControlMessageType
}

// NewInvalidControlMessageNotification returns a new *InvCtrlMsgNotif.
// Args:
// - peerID: peer ID of the sender.
// - ctlMsgType: the control message type.
// - errs: validation errors that occurred.
// Returns:
// - *InvCtrlMsgNotif: the invalid control message notification.
func NewInvalidControlMessageNotification(peerID peer.ID, ctlMsgType p2pmsg.ControlMessageType, errs InvCtrlMsgErrs) *InvCtrlMsgNotif {
	return &InvCtrlMsgNotif{
		PeerID:  peerID,
		Errors:  errs,
		MsgType: ctlMsgType,
	}
}

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

package validation

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
)

// ErrRateLimitedControlMsg indicates the specified RPC control message is rate limited for the specified peer.
type ErrRateLimitedControlMsg struct {
	controlMsg p2p.ControlMessageType
}

func (e ErrRateLimitedControlMsg) Error() string {
	return fmt.Sprintf("control message %s is rate limited for peer", e.controlMsg)
}

// NewRateLimitedControlMsgErr returns a new ErrValidationLimit.
func NewRateLimitedControlMsgErr(controlMsg p2p.ControlMessageType) ErrRateLimitedControlMsg {
	return ErrRateLimitedControlMsg{controlMsg: controlMsg}
}

// IsErrRateLimitedControlMsg returns whether an error is ErrRateLimitedControlMsg.
func IsErrRateLimitedControlMsg(err error) bool {
	var e ErrRateLimitedControlMsg
	return errors.As(err, &e)
}

// ErrDuplicateTopic error that indicates a duplicate topic in control message has been detected.
type ErrDuplicateTopic struct {
	topic channels.Topic
}

func (e ErrDuplicateTopic) Error() string {
	return fmt.Errorf("duplicate topic %s", e.topic).Error()
}

// NewDuplicateTopicErr returns a new ErrDuplicateTopic.
func NewDuplicateTopicErr(topic channels.Topic) ErrDuplicateTopic {
	return ErrDuplicateTopic{topic: topic}
}

// IsErrDuplicateTopic returns true if an error is ErrDuplicateTopic.
func IsErrDuplicateTopic(err error) bool {
	var e ErrDuplicateTopic
	return errors.As(err, &e)
}

// ErrActiveClusterIdsNotSet error that indicates a cluster prefixed control message has been received but the cluster IDs have not been set yet.
type ErrActiveClusterIdsNotSet struct {
	topic channels.Topic
}

func (e ErrActiveClusterIdsNotSet) Error() string {
	return fmt.Errorf("failed to validate cluster prefixed topic %s no active cluster IDs set", e.topic).Error()
}

// NewActiveClusterIdsNotSetErr returns a new ErrActiveClusterIdsNotSet.
func NewActiveClusterIdsNotSetErr(topic channels.Topic) ErrActiveClusterIdsNotSet {
	return ErrActiveClusterIdsNotSet{topic: topic}
}

// IsErrActiveClusterIDsNotSet returns true if an error is ErrActiveClusterIdsNotSet.
func IsErrActiveClusterIDsNotSet(err error) bool {
	var e ErrActiveClusterIdsNotSet
	return errors.As(err, &e)
}

// ErrUnstakedPeer error that indicates a cluster prefixed control message has been from an unstaked peer.
type ErrUnstakedPeer struct {
	err error
}

func (e ErrUnstakedPeer) Error() string {
	return e.err.Error()
}

// NewUnstakedPeerErr returns a new ErrUnstakedPeer.
func NewUnstakedPeerErr(err error) ErrUnstakedPeer {
	return ErrUnstakedPeer{err: err}
}

// IsErrUnstakedPeer returns true if an error is ErrUnstakedPeer.
func IsErrUnstakedPeer(err error) bool {
	var e ErrUnstakedPeer
	return errors.As(err, &e)
}

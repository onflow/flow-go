package validation

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/network/channels"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

// IWantCacheMissThresholdErr indicates that the amount of cache misses exceeds the allowed threshold.
type IWantCacheMissThresholdErr struct {
	misses          float64
	totalMessageIDS float64
	threshold       float64
}

func (e IWantCacheMissThresholdErr) Error() string {
	return fmt.Sprintf("%f/%f iWant cache misses exceeds the allowed threshold: %f", e.misses, e.totalMessageIDS, e.threshold)
}

// NewIWantCacheMissThresholdErr returns a new IWantCacheMissThresholdErr.
func NewIWantCacheMissThresholdErr(misses, totalMessageIDS, threshold float64) IWantCacheMissThresholdErr {
	return IWantCacheMissThresholdErr{misses, totalMessageIDS, threshold}
}

// IsIWantCacheMissThresholdErr returns true if an error is IWantCacheMissThresholdErr
func IsIWantCacheMissThresholdErr(err error) bool {
	var e IWantCacheMissThresholdErr
	return errors.As(err, &e)
}

// ErrHardThreshold indicates that the amount of RPC messages received exceeds hard threshold.
type ErrHardThreshold struct {
	// controlMsg the control message type.
	controlMsg p2pmsg.ControlMessageType
	// amount the amount of control messages.
	amount uint64
	// hardThreshold configured hard threshold.
	hardThreshold uint64
}

func (e ErrHardThreshold) Error() string {
	return fmt.Sprintf("number of %s messges received exceeds the configured hard threshold: received %d hard threshold %d", e.controlMsg, e.amount, e.hardThreshold)
}

// NewHardThresholdErr returns a new ErrHardThreshold.
func NewHardThresholdErr(controlMsg p2pmsg.ControlMessageType, amount, hardThreshold uint64) ErrHardThreshold {
	return ErrHardThreshold{controlMsg: controlMsg, amount: amount, hardThreshold: hardThreshold}
}

// IsErrHardThreshold returns true if an error is ErrHardThreshold
func IsErrHardThreshold(err error) bool {
	var e ErrHardThreshold
	return errors.As(err, &e)
}

// ErrRateLimitedControlMsg indicates the specified RPC control message is rate limited for the specified peer.
type ErrRateLimitedControlMsg struct {
	controlMsg p2pmsg.ControlMessageType
}

func (e ErrRateLimitedControlMsg) Error() string {
	return fmt.Sprintf("control message %s is rate limited for peer", e.controlMsg)
}

// NewRateLimitedControlMsgErr returns a new ErrValidationLimit.
func NewRateLimitedControlMsgErr(controlMsg p2pmsg.ControlMessageType) ErrRateLimitedControlMsg {
	return ErrRateLimitedControlMsg{controlMsg: controlMsg}
}

// IsErrRateLimitedControlMsg returns whether an error is ErrRateLimitedControlMsg.
func IsErrRateLimitedControlMsg(err error) bool {
	var e ErrRateLimitedControlMsg
	return errors.As(err, &e)
}

// DuplicateFoundErr error that indicates a duplicate has been detected. This can be duplicate topic or message ID tracking.
type DuplicateFoundErr struct {
	err error
}

func (e DuplicateFoundErr) Error() string {
	return e.err.Error()
}

// NewDuplicateFoundErr returns a new DuplicateFoundErr.
func NewDuplicateFoundErr(err error) DuplicateFoundErr {
	return DuplicateFoundErr{err}
}

// IsDuplicateFoundErr returns true if an error is DuplicateFoundErr.
func IsDuplicateFoundErr(err error) bool {
	var e DuplicateFoundErr
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

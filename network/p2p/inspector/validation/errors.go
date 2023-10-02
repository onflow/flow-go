package validation

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/network/channels"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

// IWantDuplicateMsgIDThresholdErr indicates that the amount of duplicate message ids exceeds the allowed threshold.
type IWantDuplicateMsgIDThresholdErr struct {
	duplicates int
	sampleSize uint
	threshold  float64
}

func (e IWantDuplicateMsgIDThresholdErr) Error() string {
	return fmt.Sprintf("%d/%d iWant duplicate message ids exceeds the allowed threshold: %f", e.duplicates, e.sampleSize, e.threshold)
}

// NewIWantDuplicateMsgIDThresholdErr returns a new IWantDuplicateMsgIDThresholdErr.
func NewIWantDuplicateMsgIDThresholdErr(duplicates int, sampleSize uint, threshold float64) IWantDuplicateMsgIDThresholdErr {
	return IWantDuplicateMsgIDThresholdErr{duplicates, sampleSize, threshold}
}

// IsIWantDuplicateMsgIDThresholdErr returns true if an error is IWantDuplicateMsgIDThresholdErr
func IsIWantDuplicateMsgIDThresholdErr(err error) bool {
	var e IWantDuplicateMsgIDThresholdErr
	return errors.As(err, &e)
}

// IWantCacheMissThresholdErr indicates that the amount of cache misses exceeds the allowed threshold.
type IWantCacheMissThresholdErr struct {
	cacheMissCount int // total iwant cache misses
	sampleSize     uint
	threshold      float64
}

func (e IWantCacheMissThresholdErr) Error() string {
	return fmt.Sprintf("%d/%d iWant cache misses exceeds the allowed threshold: %f", e.cacheMissCount, e.sampleSize, e.threshold)
}

// NewIWantCacheMissThresholdErr returns a new IWantCacheMissThresholdErr.
func NewIWantCacheMissThresholdErr(cacheMissCount int, sampleSize uint, threshold float64) IWantCacheMissThresholdErr {
	return IWantCacheMissThresholdErr{cacheMissCount, sampleSize, threshold}
}

// IsIWantCacheMissThresholdErr returns true if an error is IWantCacheMissThresholdErr
func IsIWantCacheMissThresholdErr(err error) bool {
	var e IWantCacheMissThresholdErr
	return errors.As(err, &e)
}

// DuplicateTopicErr error that indicates a duplicate has been detected. This can be duplicate topic or message ID tracking.
type DuplicateTopicErr struct {
	topic   string
	msgType p2pmsg.ControlMessageType
}

func (e DuplicateTopicErr) Error() string {
	return fmt.Sprintf("duplicate topic foud in %s control message type: %s", e.msgType, e.topic)
}

// NewDuplicateTopicErr returns a new DuplicateTopicErr.
func NewDuplicateTopicErr(topic string, msgType p2pmsg.ControlMessageType) DuplicateTopicErr {
	return DuplicateTopicErr{topic, msgType}
}

// IsDuplicateTopicErr returns true if an error is DuplicateTopicErr.
func IsDuplicateTopicErr(err error) bool {
	var e DuplicateTopicErr
	return errors.As(err, &e)
}

// DuplicateMessageIDErr error that indicates a duplicate message ID has been detected in a IHAVE or IWANT control message.
type DuplicateMessageIDErr struct {
	id      string
	msgType p2pmsg.ControlMessageType
}

func (e DuplicateMessageIDErr) Error() string {
	return fmt.Sprintf("duplicate message ID foud in %s control message type: %s", e.msgType, e.id)
}

// NewDuplicateMessageIDErr returns a new DuplicateMessageIDErr.
func NewDuplicateMessageIDErr(id string, msgType p2pmsg.ControlMessageType) DuplicateMessageIDErr {
	return DuplicateMessageIDErr{id, msgType}
}

// IsDuplicateMessageIDErr returns true if an error is DuplicateMessageIDErr.
func IsDuplicateMessageIDErr(err error) bool {
	var e DuplicateMessageIDErr
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

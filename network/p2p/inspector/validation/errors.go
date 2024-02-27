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
	threshold  int
}

func (e IWantDuplicateMsgIDThresholdErr) Error() string {
	return fmt.Sprintf("%d/%d iWant duplicate message ids exceeds the allowed threshold: %d", e.duplicates, e.sampleSize, e.threshold)
}

// NewIWantDuplicateMsgIDThresholdErr returns a new IWantDuplicateMsgIDThresholdErr.
func NewIWantDuplicateMsgIDThresholdErr(duplicates int, sampleSize uint, threshold int) IWantDuplicateMsgIDThresholdErr {
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
	threshold      int
}

func (e IWantCacheMissThresholdErr) Error() string {
	return fmt.Sprintf("%d/%d iWant cache misses exceeds the allowed threshold: %d", e.cacheMissCount, e.sampleSize, e.threshold)
}

// NewIWantCacheMissThresholdErr returns a new IWantCacheMissThresholdErr.
func NewIWantCacheMissThresholdErr(cacheMissCount int, sampleSize uint, threshold int) IWantCacheMissThresholdErr {
	return IWantCacheMissThresholdErr{cacheMissCount, sampleSize, threshold}
}

// IsIWantCacheMissThresholdErr returns true if an error is IWantCacheMissThresholdErr
func IsIWantCacheMissThresholdErr(err error) bool {
	var e IWantCacheMissThresholdErr
	return errors.As(err, &e)
}

// DuplicateTopicErr error that indicates a duplicate has been detected. This can be duplicate topic or message ID tracking.
type DuplicateTopicErr struct {
	topic   string                    // the topic that is duplicated
	count   int                       // the number of times the topic has been duplicated
	msgType p2pmsg.ControlMessageType // the control message type that the topic was found in
}

func (e DuplicateTopicErr) Error() string {
	return fmt.Sprintf("duplicate topic found in %s control message type: %s", e.msgType, e.topic)
}

// NewDuplicateTopicErr returns a new DuplicateTopicErr.
// Args:
//
//	topic: the topic that is duplicated
//	count: the number of times the topic has been duplicated
//	msgType: the control message type that the topic was found in
//
// Returns:
//
//	A new DuplicateTopicErr.
func NewDuplicateTopicErr(topic string, count int, msgType p2pmsg.ControlMessageType) DuplicateTopicErr {

	return DuplicateTopicErr{topic, count, msgType}
}

// IsDuplicateTopicErr returns true if an error is DuplicateTopicErr.
func IsDuplicateTopicErr(err error) bool {
	var e DuplicateTopicErr
	return errors.As(err, &e)
}

// DuplicateMessageIDErr error that indicates a duplicate message ID has been detected in a IHAVE or IWANT control message.
type DuplicateMessageIDErr struct {
	id      string                    // id of the message that is duplicated
	count   int                       // the number of times the message ID has been duplicated
	msgType p2pmsg.ControlMessageType // the control message type that the message ID was found in
}

func (e DuplicateMessageIDErr) Error() string {
	return fmt.Sprintf("duplicate message ID foud in %s control message type: %s", e.msgType, e.id)
}

// NewDuplicateMessageIDErr returns a new DuplicateMessageIDErr.
// Args:
//
//	id: id of the message that is duplicated
//	count: the number of times the message ID has been duplicated
//	msgType: the control message type that the message ID was found in.
func NewDuplicateMessageIDErr(id string, count int, msgType p2pmsg.ControlMessageType) DuplicateMessageIDErr {
	return DuplicateMessageIDErr{id, count, msgType}
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

// InvalidRpcPublishMessagesErr error indicates that rpc publish message validation failed.
type InvalidRpcPublishMessagesErr struct {
	// err the original error returned by the calling func.
	err error
	// count the number of times this err was encountered.
	count int
}

func (e InvalidRpcPublishMessagesErr) Error() string {
	return fmt.Errorf("rpc publish messages validation failed %d error(s) encountered: %w", e.count, e.err).Error()
}

// NewInvalidRpcPublishMessagesErr returns a new InvalidRpcPublishMessagesErr.
func NewInvalidRpcPublishMessagesErr(err error, count int) InvalidRpcPublishMessagesErr {
	return InvalidRpcPublishMessagesErr{err: err, count: count}
}

// IsInvalidRpcPublishMessagesErr returns true if an error is InvalidRpcPublishMessagesErr.
func IsInvalidRpcPublishMessagesErr(err error) bool {
	var e InvalidRpcPublishMessagesErr
	return errors.As(err, &e)
}

// DuplicateTopicIDThresholdExceeded indicates that the number of duplicate topic IDs exceeds the allowed threshold.
type DuplicateTopicIDThresholdExceeded struct {
	duplicates int
	sampleSize int
	threshold  int
}

func (e DuplicateTopicIDThresholdExceeded) Error() string {
	return fmt.Sprintf("%d/%d duplicate topic IDs exceed the allowed threshold: %d", e.duplicates, e.sampleSize, e.threshold)
}

// NewDuplicateTopicIDThresholdExceeded returns a new DuplicateTopicIDThresholdExceeded error.
func NewDuplicateTopicIDThresholdExceeded(duplicates int, sampleSize int, threshold int) DuplicateTopicIDThresholdExceeded {
	return DuplicateTopicIDThresholdExceeded{duplicates, sampleSize, threshold}
}

// IsDuplicateTopicIDThresholdExceeded returns true if an error is DuplicateTopicIDThresholdExceeded
func IsDuplicateTopicIDThresholdExceeded(err error) bool {
	var e DuplicateTopicIDThresholdExceeded
	return errors.As(err, &e)
}

// InvalidTopicIDThresholdExceeded indicates that the number of invalid topic IDs exceeds the allowed threshold.
type InvalidTopicIDThresholdExceeded struct {
	invalidCount int
	threshold    int
}

func (e InvalidTopicIDThresholdExceeded) Error() string {
	return fmt.Sprintf("%d invalid topic IDs exceed the allowed threshold: %d", e.invalidCount, e.threshold)
}

// NewInvalidTopicIDThresholdExceeded returns a new InvalidTopicIDThresholdExceeded error.
func NewInvalidTopicIDThresholdExceeded(invalidCount, threshold int) InvalidTopicIDThresholdExceeded {
	return InvalidTopicIDThresholdExceeded{invalidCount, threshold}
}

// IsInvalidTopicIDThresholdExceeded returns true if an error is InvalidTopicIDThresholdExceeded.
func IsInvalidTopicIDThresholdExceeded(err error) bool {
	var e InvalidTopicIDThresholdExceeded
	return errors.As(err, &e)
}

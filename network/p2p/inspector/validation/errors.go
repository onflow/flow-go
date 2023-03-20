package validation

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
)

// ErrDiscardThreshold indicates that the amount of RPC messages received exceeds discard threshold.
type ErrDiscardThreshold struct {
	// controlMsg the control message type.
	controlMsg p2p.ControlMessageType
	// amount the amount of control messages.
	amount uint64
	// discardThreshold configured discard threshold.
	discardThreshold uint64
}

func (e ErrDiscardThreshold) Error() string {
	return fmt.Sprintf("number of %s messges received exceeds the configured discard threshold: received %d discard threshold %d", e.controlMsg, e.amount, e.discardThreshold)
}

// NewDiscardThresholdErr returns a new ErrDiscardThreshold.
func NewDiscardThresholdErr(controlMsg p2p.ControlMessageType, amount, discardThreshold uint64) ErrDiscardThreshold {
	return ErrDiscardThreshold{controlMsg: controlMsg, amount: amount, discardThreshold: discardThreshold}
}

// IsErrDiscardThreshold returns true if an error is ErrDiscardThreshold
func IsErrDiscardThreshold(err error) bool {
	var e ErrDiscardThreshold
	return errors.As(err, &e)
}

// ErrInvalidLimitConfig indicates the validation limit is < 0.
type ErrInvalidLimitConfig struct {
	// controlMsg the control message type.
	controlMsg p2p.ControlMessageType
	// limit the value of the configuration limit.
	limit uint64
	// limitStr the string representation of the config limit.
	limitStr string
}

func (e ErrInvalidLimitConfig) Error() string {
	return fmt.Sprintf("invalid rpc control message %s validation limit %s configuration value must be greater than 0:%d", e.controlMsg, e.limitStr, e.limit)
}

// NewInvalidLimitConfigErr returns a new ErrValidationLimit.
func NewInvalidLimitConfigErr(controlMsg p2p.ControlMessageType, limitStr string, limit uint64) ErrInvalidLimitConfig {
	return ErrInvalidLimitConfig{controlMsg: controlMsg, limit: limit, limitStr: limitStr}
}

// IsErrInvalidLimitConfig returns whether an error is ErrInvalidLimitConfig
func IsErrInvalidLimitConfig(err error) bool {
	var e ErrInvalidLimitConfig
	return errors.As(err, &e)
}

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

// IsErrRateLimitedControlMsg returns whether an error is ErrRateLimitedControlMsg
func IsErrRateLimitedControlMsg(err error) bool {
	var e ErrRateLimitedControlMsg
	return errors.As(err, &e)
}

// ErrInvalidTopic error wrapper that indicates an error when checking if a Topic is a valid Flow Topic.
type ErrInvalidTopic struct {
	topic channels.Topic
	err   error
}

func (e ErrInvalidTopic) Error() string {
	return fmt.Errorf("invalid topic %s: %w", e.topic, e.err).Error()
}

// NewInvalidTopicErr returns a new ErrMalformedTopic
func NewInvalidTopicErr(topic channels.Topic, err error) ErrInvalidTopic {
	return ErrInvalidTopic{topic: topic, err: err}
}

// IsErrInvalidTopic returns true if an error is ErrInvalidTopic
func IsErrInvalidTopic(err error) bool {
	var e ErrInvalidTopic
	return errors.As(err, &e)
}

// ErrDuplicateTopic error that indicates a duplicate topic in control message has been detected.
type ErrDuplicateTopic struct {
	topic channels.Topic
}

func (e ErrDuplicateTopic) Error() string {
	return fmt.Errorf("duplicate topic %s", e.topic).Error()
}

// NewIDuplicateTopicErr returns a new ErrDuplicateTopic
func NewIDuplicateTopicErr(topic channels.Topic) ErrDuplicateTopic {
	return ErrDuplicateTopic{topic: topic}
}

// IsErrDuplicateTopic returns true if an error is ErrDuplicateTopic
func IsErrDuplicateTopic(err error) bool {
	var e ErrDuplicateTopic
	return errors.As(err, &e)
}

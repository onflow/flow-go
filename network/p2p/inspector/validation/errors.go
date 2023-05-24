package validation

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
)

// ErrHardThreshold indicates that the amount of RPC messages received exceeds hard threshold.
type ErrHardThreshold struct {
	// controlMsg the control message type.
	controlMsg p2p.ControlMessageType
	// amount the amount of control messages.
	amount uint64
	// hardThreshold configured hard threshold.
	hardThreshold uint64
}

func (e ErrHardThreshold) Error() string {
	return fmt.Sprintf("number of %s messges received exceeds the configured hard threshold: received %d hard threshold %d", e.controlMsg, e.amount, e.hardThreshold)
}

// NewHardThresholdErr returns a new ErrHardThreshold.
func NewHardThresholdErr(controlMsg p2p.ControlMessageType, amount, hardThreshold uint64) ErrHardThreshold {
	return ErrHardThreshold{controlMsg: controlMsg, amount: amount, hardThreshold: hardThreshold}
}

// IsErrHardThreshold returns true if an error is ErrHardThreshold
func IsErrHardThreshold(err error) bool {
	var e ErrHardThreshold
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
	// topic the invalid topic.
	topic channels.Topic
	// sampleSize the total amount of topics to be inspected before error is encountered.
	sampleSize uint
	// err the validation error
	err error
}

func (e ErrInvalidTopic) Error() string {
	if e.sampleSize > 0 {
		return fmt.Errorf("invalid topic %s out of %d total topics sampled: %w", e.topic, e.sampleSize, e.err).Error()
	}
	return fmt.Errorf("invalid topic %s: %w", e.topic, e.err).Error()
}

// NewInvalidTopicErr returns a new ErrMalformedTopic
func NewInvalidTopicErr(topic channels.Topic, sampleSize uint, err error) ErrInvalidTopic {
	return ErrInvalidTopic{topic: topic, sampleSize: sampleSize, err: err}
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

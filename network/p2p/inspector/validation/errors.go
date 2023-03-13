package validation

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
)

// ErrUpperThreshold indicates that the amount of RPC messages received exceeds upper threshold.
type ErrUpperThreshold struct {
	controlMsg     p2p.ControlMessageType
	amount         uint64
	upperThreshold uint64
}

func (e ErrUpperThreshold) Error() string {
	return fmt.Sprintf("number of %s messges received exceeds the configured upper threshold: received %d upper threshold %d", e.controlMsg, e.amount, e.upperThreshold)
}

// NewUpperThresholdErr returns a new ErrUpperThreshold
func NewUpperThresholdErr(controlMsg p2p.ControlMessageType, amount, upperThreshold uint64) ErrUpperThreshold {
	return ErrUpperThreshold{controlMsg: controlMsg, amount: amount, upperThreshold: upperThreshold}
}

// IsErrUpperThreshold returns true if an error is ErrUpperThreshold
func IsErrUpperThreshold(err error) bool {
	var e ErrUpperThreshold
	return errors.As(err, &e)
}

// ErrMalformedTopic indicates that the rpc control message has an invalid topic ID.
type ErrMalformedTopic struct {
	controlMsg p2p.ControlMessageType
	topic      channels.Topic
}

func (e ErrMalformedTopic) Error() string {
	return fmt.Sprintf("malformed topic ID in control message %s could not get channel from topic: %s", e.controlMsg, e.topic)
}

// NewMalformedTopicErr returns a new ErrMalformedTopic
func NewMalformedTopicErr(controlMsg p2p.ControlMessageType, topic channels.Topic) ErrMalformedTopic {
	return ErrMalformedTopic{controlMsg: controlMsg, topic: topic}
}

// IsErrMalformedTopic returns true if an error is ErrMalformedTopic
func IsErrMalformedTopic(err error) bool {
	var e ErrMalformedTopic
	return errors.As(err, &e)
}

// ErrUnknownTopicChannel indicates that the rpc control message has a topic ID associated with an unknown channel.
type ErrUnknownTopicChannel struct {
	controlMsg p2p.ControlMessageType
	topic      channels.Topic
}

func (e ErrUnknownTopicChannel) Error() string {
	return fmt.Sprintf("unknown the channel for topic ID %s in control message %s", e.topic, e.controlMsg)
}

// NewUnknownTopicChannelErr returns a new ErrMalformedTopic
func NewUnknownTopicChannelErr(controlMsg p2p.ControlMessageType, topic channels.Topic) ErrUnknownTopicChannel {
	return ErrUnknownTopicChannel{controlMsg: controlMsg, topic: topic}
}

// IsErrUnknownTopicChannel returns true if an error is ErrUnknownTopicChannel
func IsErrUnknownTopicChannel(err error) bool {
	var e ErrMalformedTopic
	return errors.As(err, &e)
}

// ErrValidationLimit indicates the validation limit is < 0.
type ErrValidationLimit struct {
	controlMsg p2p.ControlMessageType
	limit      uint64
	limitStr   string
}

func (e ErrValidationLimit) Error() string {
	return fmt.Sprintf("invalid rpc control message %s validation limit %s configuration value must be greater than 0:%d", e.controlMsg, e.limitStr, e.limit)
}

// NewValidationLimitErr returns a new ErrValidationLimit.
func NewValidationLimitErr(controlMsg p2p.ControlMessageType, limitStr string, limit uint64) ErrValidationLimit {
	return ErrValidationLimit{controlMsg: controlMsg, limit: limit, limitStr: limitStr}
}

// IsErrValidationLimit returns whether an error is ErrValidationLimit
func IsErrValidationLimit(err error) bool {
	var e ErrValidationLimit
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

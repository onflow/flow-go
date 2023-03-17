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

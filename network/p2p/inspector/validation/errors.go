package validation

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/network/channels"
)

// ErrUpperThreshold indicates that the amount of RPC messages received exceeds upper threshold.
type ErrUpperThreshold struct {
	controlMsg     ControlMsg
	amount         int
	upperThreshold int
}

func (e ErrUpperThreshold) Error() string {
	return fmt.Sprintf("number of %s messges received exceeds the configured upper threshold: received %d upper threshold %d", e.controlMsg, e.amount, e.upperThreshold)
}

// NewUpperThresholdErr returns a new ErrUpperThreshold
func NewUpperThresholdErr(controlMsg ControlMsg, amount, upperThreshold int) ErrUpperThreshold {
	return ErrUpperThreshold{controlMsg: controlMsg, amount: amount, upperThreshold: upperThreshold}
}

// IsErrUpperThreshold returns true if an error is ErrUpperThreshold
func IsErrUpperThreshold(err error) bool {
	var e ErrUpperThreshold
	return errors.As(err, &e)
}

// ErrMalformedTopic indicates that the rpc control message has an invalid topic ID.
type ErrMalformedTopic struct {
	controlMsg ControlMsg
	topic      channels.Topic
}

func (e ErrMalformedTopic) Error() string {
	return fmt.Sprintf("malformed topic ID in control message %s could not get channel from topic: %s", e.controlMsg, e.topic)
}

// NewMalformedTopicErr returns a new ErrMalformedTopic
func NewMalformedTopicErr(controlMsg ControlMsg, topic channels.Topic) ErrMalformedTopic {
	return ErrMalformedTopic{controlMsg: controlMsg, topic: topic}
}

// IsErrMalformedTopic returns true if an error is ErrMalformedTopic
func IsErrMalformedTopic(err error) bool {
	var e ErrMalformedTopic
	return errors.As(err, &e)
}

// ErrUnknownTopicChannel indicates that the rpc control message has a topic ID associated with an unknown channel.
type ErrUnknownTopicChannel struct {
	controlMsg ControlMsg
	topic      channels.Topic
}

func (e ErrUnknownTopicChannel) Error() string {
	return fmt.Sprintf("the channel for topic ID %s in control message %s does not exist", e.topic, e.controlMsg)
}

// NewUnknownTopicChannelErr returns a new ErrMalformedTopic
func NewUnknownTopicChannelErr(controlMsg ControlMsg, topic channels.Topic) ErrUnknownTopicChannel {
	return ErrUnknownTopicChannel{controlMsg: controlMsg, topic: topic}
}

// IsErrUnknownTopicChannel returns true if an error is ErrUnknownTopicChannel
func IsErrUnknownTopicChannel(err error) bool {
	var e ErrMalformedTopic
	return errors.As(err, &e)
}

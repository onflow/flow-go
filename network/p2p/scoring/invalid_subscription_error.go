package scoring

import (
	"errors"
	"fmt"
)

// InvalidSubscriptionError indicates that a peer has subscribed to a topic that is not allowed for its role.
type InvalidSubscriptionError struct {
	topic string // the topic that the peer is subscribed to, but not allowed to.
}

func NewInvalidSubscriptionError(topic string) *InvalidSubscriptionError {
	return &InvalidSubscriptionError{
		topic: topic,
	}
}

func (e InvalidSubscriptionError) Error() string {
	return fmt.Sprintf("unauthorized subscription: %s", e.topic)
}

func IsInvalidSubscriptionError(this error) bool {
	var e InvalidSubscriptionError
	return errors.As(this, &e)
}

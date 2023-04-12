package channels

import (
	"errors"
	"fmt"
)

// ErrInvalidTopic error wrapper that indicates an error when checking if a Topic is a valid Flow Topic.
type ErrInvalidTopic struct {
	topic Topic
	err   error
}

func (e ErrInvalidTopic) Error() string {
	return fmt.Errorf("invalid topic %s: %w", e.topic, e.err).Error()
}

// NewInvalidTopicErr returns a new ErrMalformedTopic
func NewInvalidTopicErr(topic Topic, err error) ErrInvalidTopic {
	return ErrInvalidTopic{topic: topic, err: err}
}

// IsErrInvalidTopic returns true if an error is ErrInvalidTopic
func IsErrInvalidTopic(err error) bool {
	var e ErrInvalidTopic
	return errors.As(err, &e)
}

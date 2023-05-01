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

// ErrUnknownClusterID error wrapper that indicates an invalid topic with an unknown cluster ID prefix.
type ErrUnknownClusterID struct {
	topic            Topic
	clusterID        string
	activeClusterIDS []string
}

func (e ErrUnknownClusterID) Error() string {
	return fmt.Errorf("cluster ID %s for topic %s not found in active cluster IDs list %s", e.clusterID, e.topic, e.activeClusterIDS).Error()
}

// NewUnknownClusterIDErr returns a new ErrUnknownClusterID
func NewUnknownClusterIDErr(clusterID string, activeClusterIDS []string) ErrUnknownClusterID {
	return ErrUnknownClusterID{clusterID: clusterID, activeClusterIDS: activeClusterIDS}
}

// IsErrUnknownClusterID returns true if an error is ErrUnknownClusterID
func IsErrUnknownClusterID(err error) bool {
	var e ErrUnknownClusterID
	return errors.As(err, &e)
}

package channels

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
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
	clusterId        flow.ChainID
	activeClusterIds flow.ChainIDList
}

func (e ErrUnknownClusterID) Error() string {
	return fmt.Errorf("cluster ID %s not found in active cluster IDs list %s", e.clusterId, e.activeClusterIds).Error()
}

// NewUnknownClusterIdErr returns a new ErrUnknownClusterID
func NewUnknownClusterIdErr(clusterId flow.ChainID, activeClusterIds flow.ChainIDList) ErrUnknownClusterID {
	return ErrUnknownClusterID{clusterId: clusterId, activeClusterIds: activeClusterIds}
}

// IsErrUnknownClusterID returns true if an error is ErrUnknownClusterID
func IsErrUnknownClusterID(err error) bool {
	var e ErrUnknownClusterID
	return errors.As(err, &e)
}

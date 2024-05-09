package channels

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// InvalidTopicErr error wrapper that indicates an error when checking if a Topic is a valid Flow Topic.
type InvalidTopicErr struct {
	topic Topic
	err   error
}

func (e InvalidTopicErr) Error() string {
	return fmt.Errorf("invalid topic %s: %w", e.topic, e.err).Error()
}

// NewInvalidTopicErr returns a new ErrMalformedTopic
func NewInvalidTopicErr(topic Topic, err error) InvalidTopicErr {
	return InvalidTopicErr{topic: topic, err: err}
}

// IsInvalidTopicErr returns true if an error is InvalidTopicErr
func IsInvalidTopicErr(err error) bool {
	var e InvalidTopicErr
	return errors.As(err, &e)
}

// UnknownClusterIDErr error wrapper that indicates an invalid topic with an unknown cluster ID prefix.
type UnknownClusterIDErr struct {
	clusterId        flow.ChainID
	activeClusterIds flow.ChainIDList
}

func (e UnknownClusterIDErr) Error() string {
	return fmt.Errorf("cluster ID %s not found in active cluster IDs list %s", e.clusterId, e.activeClusterIds).Error()
}

// NewUnknownClusterIdErr returns a new UnknownClusterIDErr
func NewUnknownClusterIdErr(clusterId flow.ChainID, activeClusterIds flow.ChainIDList) UnknownClusterIDErr {
	return UnknownClusterIDErr{clusterId: clusterId, activeClusterIds: activeClusterIds}
}

// IsUnknownClusterIDErr returns true if an error is UnknownClusterIDErr
func IsUnknownClusterIDErr(err error) bool {
	var e UnknownClusterIDErr
	return errors.As(err, &e)
}

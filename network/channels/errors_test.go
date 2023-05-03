package channels

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestErrInvalidTopicRoundTrip ensures correct error formatting for ErrInvalidTopic.
func TestErrInvalidTopicRoundTrip(t *testing.T) {
	topic := Topic("invalid-topic")
	wrapErr := fmt.Errorf("this err should be wrapped with topic to add context")
	err := NewInvalidTopicErr(topic, wrapErr)

	// tests the error message formatting.
	expectedErrMsg := fmt.Errorf("invalid topic %s: %w", topic, wrapErr).Error()
	assert.Equal(t, expectedErrMsg, err.Error(), "the error message should be correctly formatted")

	// tests the IsErrActiveClusterIDsNotSet function.
	assert.True(t, IsErrInvalidTopic(err), "IsErrInvalidTopic should return true for ErrInvalidTopic error")

	// test IsErrActiveClusterIDsNotSet with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, IsErrInvalidTopic(dummyErr), "IsErrInvalidTopic should return false for non-IsErrInvalidTopic error")
}

// TestErrUnknownClusterIDRoundTrip ensures correct error formatting for ErrUnknownClusterID.
func TestErrUnknownClusterIDRoundTrip(t *testing.T) {
	clusterId := "cluster-id"
	activeClusterIds := []string{"active", "cluster", "ids"}
	err := NewUnknownClusterIdErr(clusterId, activeClusterIds)

	// tests the error message formatting.
	expectedErrMsg := fmt.Errorf("cluster ID %s not found in active cluster IDs list %s", clusterId, activeClusterIds).Error()
	assert.Equal(t, expectedErrMsg, err.Error(), "the error message should be correctly formatted")

	// tests the IsErrActiveClusterIDsNotSet function.
	assert.True(t, IsErrUnknownClusterID(err), "IsErrUnknownClusterID should return true for ErrUnknownClusterID error")

	// test IsErrActiveClusterIDsNotSet with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, IsErrUnknownClusterID(dummyErr), "IsErrUnknownClusterID should return false for non-ErrUnknownClusterID error")
}

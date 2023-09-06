package channels

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
)

// TestInvalidTopicErrRoundTrip ensures correct error formatting for InvalidTopicErr.
func TestInvalidTopicErrRoundTrip(t *testing.T) {
	topic := Topic("invalid-topic")
	wrapErr := fmt.Errorf("this err should be wrapped with topic to add context")
	err := NewInvalidTopicErr(topic, wrapErr)

	// tests the error message formatting.
	expectedErrMsg := fmt.Errorf("invalid topic %s: %w", topic, wrapErr).Error()
	assert.Equal(t, expectedErrMsg, err.Error(), "the error message should be correctly formatted")

	// tests the IsErrActiveClusterIDsNotSet function.
	assert.True(t, IsInvalidTopicErr(err), "IsInvalidTopicErr should return true for InvalidTopicErr error")

	// test IsErrActiveClusterIDsNotSet with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, IsInvalidTopicErr(dummyErr), "IsInvalidTopicErr should return false for non-IsInvalidTopicErr error")
}

// TestUnknownClusterIDErrRoundTrip ensures correct error formatting for UnknownClusterIDErr.
func TestUnknownClusterIDErrRoundTrip(t *testing.T) {
	clusterId := flow.ChainID("cluster-id")
	activeClusterIds := flow.ChainIDList{"active", "cluster", "ids"}
	err := NewUnknownClusterIdErr(clusterId, activeClusterIds)

	// tests the error message formatting.
	expectedErrMsg := fmt.Errorf("cluster ID %s not found in active cluster IDs list %s", clusterId, activeClusterIds).Error()
	assert.Equal(t, expectedErrMsg, err.Error(), "the error message should be correctly formatted")

	// tests the IsErrActiveClusterIDsNotSet function.
	assert.True(t, IsUnknownClusterIDErr(err), "IsUnknownClusterIDErr should return true for UnknownClusterIDErr error")

	// test IsErrActiveClusterIDsNotSet with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, IsUnknownClusterIDErr(dummyErr), "IsUnknownClusterIDErr should return false for non-UnknownClusterIDErr error")
}

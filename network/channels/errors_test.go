package channels_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestInvalidTopicErrRoundTrip ensures correct error formatting for InvalidTopicErr.
func TestInvalidTopicErrRoundTrip(t *testing.T) {
	topic := channels.Topic("invalid-topic")
	wrapErr := fmt.Errorf("this err should be wrapped with topic to add context")
	err := channels.NewInvalidTopicErr(topic, wrapErr)

	// tests the error message formatting.
	expectedErrMsg := fmt.Errorf("invalid topic %s: %w", topic, wrapErr).Error()
	assert.Equal(t, expectedErrMsg, err.Error(), "the error message should be correctly formatted")

	// tests the IsErrActiveClusterIDsNotSet function.
	assert.True(t, channels.IsInvalidTopicErr(err), "IsInvalidTopicErr should return true for InvalidTopicErr error")

	// test IsErrActiveClusterIDsNotSet with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, channels.IsInvalidTopicErr(dummyErr), "IsInvalidTopicErr should return false for non-IsInvalidTopicErr error")
}

// TestUnknownClusterIDErrRoundTrip ensures correct error formatting for UnknownClusterIDErr.
func TestUnknownClusterIDErrRoundTrip(t *testing.T) {
	clusterId := cluster.CanonicalClusterID(10, unittest.IdentifierListFixture(1))
	activeClusterIds := flow.ChainIDList{
		cluster.CanonicalClusterID(3, unittest.IdentifierListFixture(1)),
		cluster.CanonicalClusterID(3, unittest.IdentifierListFixture(1)),
		cluster.CanonicalClusterID(3, unittest.IdentifierListFixture(1)),
	}
	err := channels.NewUnknownClusterIdErr(clusterId, activeClusterIds)

	// tests the error message formatting.
	expectedErrMsg := fmt.Errorf("cluster ID %s not found in active cluster IDs list %s", clusterId, activeClusterIds).Error()
	assert.Equal(t, expectedErrMsg, err.Error(), "the error message should be correctly formatted")

	// tests the IsErrActiveClusterIDsNotSet function.
	assert.True(t, channels.IsUnknownClusterIDErr(err), "IsUnknownClusterIDErr should return true for UnknownClusterIDErr error")

	// test IsErrActiveClusterIDsNotSet with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, channels.IsUnknownClusterIDErr(dummyErr), "IsUnknownClusterIDErr should return false for non-UnknownClusterIDErr error")
}

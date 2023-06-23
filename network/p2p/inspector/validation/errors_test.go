package validation

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/network/channels"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

// TestErrActiveClusterIDsNotSetRoundTrip ensures correct error formatting for ErrActiveClusterIdsNotSet.
func TestErrActiveClusterIDsNotSetRoundTrip(t *testing.T) {
	topic := channels.Topic("test-topic")
	err := NewActiveClusterIdsNotSetErr(topic)

	// tests the error message formatting.
	expectedErrMsg := fmt.Errorf("failed to validate cluster prefixed topic %s no active cluster IDs set", topic).Error()
	assert.Equal(t, expectedErrMsg, err.Error(), "the error message should be correctly formatted")

	// tests the IsErrActiveClusterIDsNotSet function.
	assert.True(t, IsErrActiveClusterIDsNotSet(err), "IsErrActiveClusterIDsNotSet should return true for ErrActiveClusterIdsNotSet error")

	// test IsErrActiveClusterIDsNotSet with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, IsErrActiveClusterIDsNotSet(dummyErr), "IsErrActiveClusterIDsNotSet should return false for non-ErrActiveClusterIdsNotSet error")
}

// TestErrHardThresholdRoundTrip ensures correct error formatting for ErrHardThreshold.
func TestErrHardThresholdRoundTrip(t *testing.T) {
	controlMsg := p2pmsg.CtrlMsgGraft
	amount := uint64(100)
	hardThreshold := uint64(500)
	err := NewHardThresholdErr(controlMsg, amount, hardThreshold)

	// tests the error message formatting.
	expectedErrMsg := fmt.Sprintf("number of %s messges received exceeds the configured hard threshold: received %d hard threshold %d", controlMsg, amount, hardThreshold)
	assert.Equal(t, expectedErrMsg, err.Error(), "the error message should be correctly formatted")

	// tests the IsErrHardThreshold function.
	assert.True(t, IsErrHardThreshold(err), "IsErrHardThreshold should return true for ErrHardThreshold error")

	// test IsErrHardThreshold with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, IsErrHardThreshold(dummyErr), "IsErrHardThreshold should return false for non-ErrHardThreshold error")
}

// TestErrRateLimitedControlMsgRoundTrip ensures correct error formatting for ErrRateLimitedControlMsg.
func TestErrRateLimitedControlMsgRoundTrip(t *testing.T) {
	controlMsg := p2pmsg.CtrlMsgGraft
	err := NewRateLimitedControlMsgErr(controlMsg)

	// tests the error message formatting.
	expectedErrMsg := fmt.Sprintf("control message %s is rate limited for peer", controlMsg)
	assert.Equal(t, expectedErrMsg, err.Error(), "the error message should be correctly formatted")

	// tests the IsErrRateLimitedControlMsg function.
	assert.True(t, IsErrRateLimitedControlMsg(err), "IsErrRateLimitedControlMsg should return true for ErrRateLimitedControlMsg error")

	// test IsErrRateLimitedControlMsg with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, IsErrRateLimitedControlMsg(dummyErr), "IsErrRateLimitedControlMsg should return false for non-ErrRateLimitedControlMsg error")
}

// TestErrDuplicateTopicRoundTrip ensures correct error formatting for ErrDuplicateTopic.
func TestErrDuplicateTopicRoundTrip(t *testing.T) {
	topic := channels.Topic("topic")
	err := NewDuplicateTopicErr(topic)

	// tests the error message formatting.
	expectedErrMsg := fmt.Errorf("duplicate topic %s", topic).Error()
	assert.Equal(t, expectedErrMsg, err.Error(), "the error message should be correctly formatted")

	// tests the IsErrDuplicateTopic function.
	assert.True(t, IsErrDuplicateTopic(err), "IsErrDuplicateTopic should return true for ErrDuplicateTopic error")

	// test IsErrDuplicateTopic with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, IsErrDuplicateTopic(dummyErr), "IsErrDuplicateTopic should return false for non-ErrDuplicateTopic error")
}

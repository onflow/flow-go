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

// TestErrDuplicateTopicRoundTrip ensures correct error formatting for DuplicateTopicErr.
func TestDuplicateTopicErrRoundTrip(t *testing.T) {
	expectedErrorMsg := fmt.Sprintf("duplicate topic found in %s control message type: %s", p2pmsg.CtrlMsgGraft, channels.TestNetworkChannel)
	err := NewDuplicateTopicErr(channels.TestNetworkChannel.String(), 1, p2pmsg.CtrlMsgGraft)
	assert.Equal(t, expectedErrorMsg, err.Error(), "the error message should be correctly formatted")
	// tests the IsDuplicateTopicErr function.
	assert.True(t, IsDuplicateTopicErr(err), "IsDuplicateTopicErr should return true for DuplicateTopicErr error")
	// test IsDuplicateTopicErr with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, IsDuplicateTopicErr(dummyErr), "IsDuplicateTopicErr should return false for non-DuplicateTopicErr error")
}

// TestErrDuplicateTopicRoundTrip ensures correct error formatting for DuplicateTopicErr.
func TestDuplicateMessageIDErrRoundTrip(t *testing.T) {
	msgID := "flow-1804flkjnafo"
	expectedErrMsg1 := fmt.Sprintf("duplicate message ID foud in %s control message type: %s", p2pmsg.CtrlMsgIHave, msgID)
	expectedErrMsg2 := fmt.Sprintf("duplicate message ID foud in %s control message type: %s", p2pmsg.CtrlMsgIWant, msgID)
	err := NewDuplicateMessageIDErr(msgID, 1, p2pmsg.CtrlMsgIHave)
	assert.Equal(t, expectedErrMsg1, err.Error(), "the error message should be correctly formatted")
	// tests the IsDuplicateTopicErr function.
	assert.True(t, IsDuplicateMessageIDErr(err), "IsDuplicateMessageIDErr should return true for DuplicateMessageIDErr error")
	err = NewDuplicateMessageIDErr(msgID, 1, p2pmsg.CtrlMsgIWant)
	assert.Equal(t, expectedErrMsg2, err.Error(), "the error message should be correctly formatted")
	// tests the IsDuplicateTopicErr function.
	assert.True(t, IsDuplicateMessageIDErr(err), "IsDuplicateMessageIDErr should return true for DuplicateMessageIDErr error")
	// test IsDuplicateMessageIDErr with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, IsDuplicateMessageIDErr(dummyErr), "IsDuplicateMessageIDErr should return false for non-DuplicateMessageIDErr error")
}

// TestIWantCacheMissThresholdErrRoundTrip ensures correct error formatting for IWantCacheMissThresholdErr.
func TestIWantCacheMissThresholdErrRoundTrip(t *testing.T) {
	err := NewIWantCacheMissThresholdErr(5, 10, 5)

	// tests the error message formatting.
	expectedErrMsg := "5/10 iWant cache misses exceeds the allowed threshold: 5"
	assert.Equal(t, expectedErrMsg, err.Error(), "the error message should be correctly formatted")

	// tests the IsIWantCacheMissThresholdErr function.
	assert.True(t, IsIWantCacheMissThresholdErr(err), "IsIWantCacheMissThresholdErr should return true for IWantCacheMissThresholdErr error")

	// test IsIWantCacheMissThresholdErr with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, IsIWantCacheMissThresholdErr(dummyErr), "IsIWantCacheMissThresholdErr should return false for non-IWantCacheMissThresholdErr error")
}

// TestIWantDuplicateMsgIDThresholdErrRoundTrip ensures correct error formatting for IWantDuplicateMsgIDThresholdErr.
func TestIWantDuplicateMsgIDThresholdErrRoundTrip(t *testing.T) {
	err := NewIWantDuplicateMsgIDThresholdErr(5, 10, 5)

	// tests the error message formatting.
	expectedErrMsg := "5/10 iWant duplicate message ids exceeds the allowed threshold: 5"
	assert.Equal(t, expectedErrMsg, err.Error(), "the error message should be correctly formatted")

	// tests the IsIWantDuplicateMsgIDThresholdErr function.
	assert.True(t, IsIWantDuplicateMsgIDThresholdErr(err), "IsIWantDuplicateMsgIDThresholdErr should return true for IWantDuplicateMsgIDThresholdErr error")

	// test IsIWantDuplicateMsgIDThresholdErr with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, IsIWantDuplicateMsgIDThresholdErr(dummyErr), "IsIWantDuplicateMsgIDThresholdErr should return false for non-IWantDuplicateMsgIDThresholdErr error")
}

// TestInvalidRpcPublishMessagesErrRoundTrip ensures correct error formatting for InvalidRpcPublishMessagesErr.
func TestInvalidRpcPublishMessagesErrRoundTrip(t *testing.T) {
	wrappedErr := fmt.Errorf("invalid topic")
	err := NewInvalidRpcPublishMessagesErr(wrappedErr, 1)

	// tests the error message formatting.
	expectedErrMsg := "rpc publish messages validation failed 1 error(s) encountered: invalid topic"
	assert.Equal(t, expectedErrMsg, err.Error(), "the error message should be correctly formatted")

	// tests the IsInvalidRpcPublishMessagesErr function.
	assert.True(t, IsInvalidRpcPublishMessagesErr(err), "IsInvalidRpcPublishMessagesErr should return true for InvalidRpcPublishMessagesErr error")

	// test IsInvalidRpcPublishMessagesErr with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, IsInvalidRpcPublishMessagesErr(dummyErr), "IsInvalidRpcPublishMessagesErr should return false for non-InvalidRpcPublishMessagesErr error")
}

// TestErrDuplicateTopicIDThresholdExceededRoundTrip ensures correct error formatting for DuplicateTopicIDThresholdExceeded.
func TestDuplicateTopicIDThresholdExceededRoundTrip(t *testing.T) {
	expectedErrorMsg := "3/5 duplicate topic IDs exceed the allowed threshold: 2"
	err := NewDuplicateTopicIDThresholdExceeded(3, 5, 2)
	assert.Equal(t, expectedErrorMsg, err.Error(), "the error message should be correctly formatted")
	// tests the IsDuplicateTopicIDThresholdExceeded function.
	assert.True(t, IsDuplicateTopicIDThresholdExceeded(err), "IsDuplicateTopicIDThresholdExceeded should return true for DuplicateTopicIDThresholdExceeded error")
	// test IsDuplicateTopicIDThresholdExceeded with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, IsDuplicateTopicIDThresholdExceeded(dummyErr), "IsDuplicateTopicIDThresholdExceeded should return false for non-DuplicateTopicIDThresholdExceeded error")
}

// TestErrInvalidTopicIDThresholdExceededRoundTrip ensures correct error formatting for InvalidTopicIDThresholdExceeded.
func TestInvalidTopicIDThresholdExceededRoundTrip(t *testing.T) {
	expectedErrorMsg := "8 invalid topic IDs exceed the allowed threshold: 5"
	err := NewInvalidTopicIDThresholdExceeded(8, 5)
	assert.Equal(t, expectedErrorMsg, err.Error(), "the error message should be correctly formatted")
	// tests the IsInvalidTopicIDThresholdExceeded function.
	assert.True(t, IsInvalidTopicIDThresholdExceeded(err), "IsInvalidTopicIDThresholdExceeded should return true for InvalidTopicIDThresholdExceeded error")
	// test IsInvalidTopicIDThresholdExceeded with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, IsInvalidTopicIDThresholdExceeded(dummyErr), "IsInvalidTopicIDThresholdExceeded should return false for non-InvalidTopicIDThresholdExceeded error")
}

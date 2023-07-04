package p2pconf

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
)

// TestErrInvalidLimitConfigRoundTrip ensures correct error formatting for ErrInvalidLimitConfig.
func TestErrInvalidLimitConfigRoundTrip(t *testing.T) {
	controlMsg := p2pmsg.CtrlMsgGraft
	limit := uint64(500)

	e := fmt.Errorf("invalid rate limit value %d must be greater than 0", limit)
	err := NewInvalidLimitConfigErr(controlMsg, e)

	// tests the error message formatting.
	expectedErrMsg := fmt.Errorf("invalid rpc control message %s validation limit configuration: %w", controlMsg, e).Error()
	assert.Equal(t, expectedErrMsg, err.Error(), "the error message should be correctly formatted")

	// tests the IsInvalidLimitConfigError function.
	assert.True(t, IsInvalidLimitConfigError(err), "IsInvalidLimitConfigError should return true for ErrInvalidLimitConfig error")

	// test IsInvalidLimitConfigError with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, IsInvalidLimitConfigError(dummyErr), "IsInvalidLimitConfigError should return false for non-ErrInvalidLimitConfig error")
}

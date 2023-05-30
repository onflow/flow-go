package network

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/network/p2p"
)

// TestErrInvalidLimitConfigRoundTrip ensures correct error formatting for ErrInvalidLimitConfig.
func TestErrInvalidLimitConfigRoundTrip(t *testing.T) {
	controlMsg := p2p.CtrlMsgGraft
	limit := uint64(500)

	e := fmt.Errorf("invalid rate limit value %d must be greater than 0", limit)
	err := NewInvalidLimitConfigErr(controlMsg, e)

	// tests the error message formatting.
	expectedErrMsg := fmt.Errorf("invalid rpc control message %s validation limit configuration: %w", controlMsg, e).Error()
	assert.Equal(t, expectedErrMsg, err.Error(), "the error message should be correctly formatted")

	// tests the IsErrInvalidLimitConfig function.
	assert.True(t, IsErrInvalidLimitConfig(err), "IsErrInvalidLimitConfig should return true for ErrInvalidLimitConfig error")

	// test IsErrInvalidLimitConfig with a different error type.
	dummyErr := fmt.Errorf("dummy error")
	assert.False(t, IsErrInvalidLimitConfig(dummyErr), "IsErrInvalidLimitConfig should return false for non-ErrInvalidLimitConfig error")
}

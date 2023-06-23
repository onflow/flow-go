package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsRetryablelConflictError(t *testing.T) {
	require.False(t, IsRetryableConflictError(fmt.Errorf("generic error")))

	err := NewRetryableConflictError("bad %s", "conflict")
	require.True(t, IsRetryableConflictError(err))

	require.True(t, IsRetryableConflictError(fmt.Errorf("wrapped: %w", err)))
}

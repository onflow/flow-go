package parser

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
)

// TestParseBlockStatus_Invalid tests the ParseBlockStatus function with invalid inputs.
// It verifies that for each invalid block status string, the function returns an error
// matching the expected error message format.
func TestParseBlockStatus_Invalid(t *testing.T) {
	tests := []string{"unknown", "pending", ""}
	expectedErr := fmt.Sprintf("invalid 'block_status', must be '%s' or '%s'", Finalized, Sealed)

	for _, input := range tests {
		_, err := ParseBlockStatus(input)
		assert.EqualError(t, err, expectedErr)
	}
}

// TestParseBlockStatus_Valid tests the ParseBlockStatus function with valid inputs.
// It ensures that the function returns the correct flow.BlockStatus for valid status
// strings "finalized" and "sealed" without errors.
func TestParseBlockStatus_Valid(t *testing.T) {
	tests := map[string]flow.BlockStatus{
		Finalized: flow.BlockStatusFinalized,
		Sealed:    flow.BlockStatusSealed,
	}

	for input, expectedStatus := range tests {
		status, err := ParseBlockStatus(input)
		assert.NoError(t, err)
		assert.Equal(t, expectedStatus, status)
	}
}

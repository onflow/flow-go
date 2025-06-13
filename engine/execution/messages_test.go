package execution_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/testutil"
)

// TestNewComputationResult verifies the behavior of the NewComputationResult constructor.
// It ensures proper handling of both valid and invalid untrusted input fields.
//
// Test Cases:
//
// 1. Valid input returns result:
//   - Verifies that a properly created UntrustedComputationResult leads to a valid ComputationResult.
//
// 2. Nil BlockExecutionResult:
//   - Ensures an error is returned when the BlockExecutionResult is nil.
//
// 3. Nil BlockAttestationResult:
//   - Ensures an error is returned when the BlockAttestationResult is nil.
func TestNewComputationResult(t *testing.T) {
	result := testutil.ComputationResultFixture(t)

	t.Run("valid input", func(t *testing.T) {
		res, err := execution.NewComputationResult(
			execution.UntrustedComputationResult{
				BlockExecutionResult:   result.BlockExecutionResult,
				BlockAttestationResult: result.BlockAttestationResult,
				ExecutionReceipt:       result.ExecutionReceipt,
			},
		)
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("nil BlockExecutionResult", func(t *testing.T) {
		res, err := execution.NewComputationResult(
			execution.UntrustedComputationResult{
				BlockExecutionResult:   nil,
				BlockAttestationResult: result.BlockAttestationResult,
				ExecutionReceipt:       result.ExecutionReceipt,
			},
		)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "block execution result must not be nil")
	})

	t.Run("nil BlockAttestationResult", func(t *testing.T) {
		res, err := execution.NewComputationResult(
			execution.UntrustedComputationResult{
				BlockExecutionResult:   result.BlockExecutionResult,
				BlockAttestationResult: nil,
				ExecutionReceipt:       result.ExecutionReceipt,
			},
		)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "block attestation result must not be nil")
	})
}

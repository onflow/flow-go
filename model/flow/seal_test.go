package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test_SealID checks that two seals that only differ in their approval
// signatures have different IDs. This is REQUIRED FOR the STORAGE layer
// to correctly retrieve the block payload!
func Test_SealID(t *testing.T) {

	seal := unittest.Seal.Fixture()
	id := seal.ID()
	cs := seal.Checksum()

	// Change signatures of first chunk
	seal.AggregatedApprovalSigs[0] = unittest.Seal.AggregatedSignatureFixture()

	// They should not have changed
	assert.NotEqual(t, id, seal.ID())
	assert.NotEqual(t, cs, seal.Checksum())
}

// TestNewSeal verifies the behavior of the NewSeal constructor.
// It ensures proper handling of both valid and invalid untrusted input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UntrustedSeal results in a valid Seal.
//
// 2. Invalid input with zero block ID:
//   - Ensures an error is returned when the BlockID is zero.
//
// 3. Invalid input with zero result ID:
//   - Ensures an error is returned when the ResultID is zero.
//
// 4. Invalid input with empty final state:
//   - Ensures an error is returned when the FinalState is empty.
func TestNewSeal(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		seal, err := flow.NewSeal(
			flow.UntrustedSeal{
				BlockID:                unittest.IdentifierFixture(),
				ResultID:               unittest.IdentifierFixture(),
				FinalState:             unittest.StateCommitmentFixture(),
				AggregatedApprovalSigs: unittest.Seal.AggregatedSignatureFixtures(3),
			},
		)
		require.NoError(t, err)
		require.NotNil(t, seal)
	})

	t.Run("invalid input, block ID is zero", func(t *testing.T) {
		seal, err := flow.NewSeal(
			flow.UntrustedSeal{
				BlockID:                flow.ZeroID,
				ResultID:               unittest.IdentifierFixture(),
				FinalState:             unittest.StateCommitmentFixture(),
				AggregatedApprovalSigs: unittest.Seal.AggregatedSignatureFixtures(3),
			},
		)
		require.Error(t, err)
		require.Nil(t, seal)
		assert.Contains(t, err.Error(), "block ID must not be zero")
	})

	t.Run("invalid input, result ID is zero", func(t *testing.T) {
		seal, err := flow.NewSeal(
			flow.UntrustedSeal{
				BlockID:                unittest.IdentifierFixture(),
				ResultID:               flow.ZeroID,
				FinalState:             unittest.StateCommitmentFixture(),
				AggregatedApprovalSigs: unittest.Seal.AggregatedSignatureFixtures(3),
			},
		)
		require.Error(t, err)
		require.Nil(t, seal)
		assert.Contains(t, err.Error(), "result ID must not be zero")
	})

	t.Run("invalid input, final state is empty", func(t *testing.T) {
		seal, err := flow.NewSeal(
			flow.UntrustedSeal{
				BlockID:                unittest.IdentifierFixture(),
				ResultID:               unittest.IdentifierFixture(),
				FinalState:             flow.EmptyStateCommitment,
				AggregatedApprovalSigs: unittest.Seal.AggregatedSignatureFixtures(3),
			},
		)
		require.Error(t, err)
		require.Nil(t, seal)
		assert.Contains(t, err.Error(), "final state must not be empty")
	})
}

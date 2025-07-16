package cluster_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewPayload verifies the behavior of the NewPayload constructor.
// It ensures proper handling of both valid and invalid untrusted input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UntrustedPayload results in a valid Payload.
//
// 2. Valid input with zero reference block ID:
//   - Ensures that Payload is still constructed when reference block ID is zero,
//     since it's allowed only for root blocks (validation should happen elsewhere).
//
// 3. Invalid input with malformed collection:
//   - Ensures an error is returned when the Collection contains invalid transaction IDs.
func TestNewPayload(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		payload := unittest.ClusterPayloadFixture(5)

		res, err := cluster.NewPayload(cluster.UntrustedPayload(*payload))
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("valid input with zero ReferenceBlockID (root block)", func(t *testing.T) {
		payload := unittest.ClusterPayloadFixture(5)
		payload.ReferenceBlockID = flow.ZeroID

		res, err := cluster.NewPayload(cluster.UntrustedPayload(*payload))
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, flow.ZeroID, res.ReferenceBlockID)
	})

	t.Run("invalid input with malformed collection", func(t *testing.T) {
		payload := unittest.ClusterPayloadFixture(5)
		collection := unittest.CollectionFixture(5)
		collection.Transactions[2] = nil
		payload.Collection = collection

		res, err := cluster.NewPayload(cluster.UntrustedPayload(*payload))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "could not construct collection")
	})
}

// TestNewRootPayload verifies the behavior of the NewRootPayload constructor.
// It ensures proper handling of both valid and invalid untrusted input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UntrustedPayload results in a valid root Payload.
//
// 2. Valid input with non-zero reference block ID:
//   - Ensures an error is returned when reference block ID is not flow.ZeroID.
//
// 3. Invalid input with non-empty collection:
//   - Ensures an error is returned when the Collection contains transaction IDs.
func TestNewRootPayload(t *testing.T) {
	// validRootPayloadFixture returns a new valid root cluster.UntrustedPayload for use in tests.
	validRootPayloadFixture := func() cluster.UntrustedPayload {
		return cluster.UntrustedPayload{
			ReferenceBlockID: flow.ZeroID,
			Collection:       *flow.NewEmptyCollection(),
		}
	}

	t.Run("valid input", func(t *testing.T) {
		res, err := cluster.NewRootPayload(validRootPayloadFixture())
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("valid input with non-zero ReferenceBlockID", func(t *testing.T) {
		payload := validRootPayloadFixture()
		payload.ReferenceBlockID = unittest.IdentifierFixture()

		res, err := cluster.NewRootPayload(payload)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "ReferenceBlockID must be empty")
	})

	t.Run("invalid input with collection", func(t *testing.T) {
		payload := validRootPayloadFixture()
		payload.Collection = unittest.CollectionFixture(5)

		res, err := cluster.NewRootPayload(payload)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "Collection must be empty")
	})
}

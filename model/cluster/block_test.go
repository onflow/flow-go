package cluster_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestClusterBlockMalleability checks that cluster.UnsignedBlock is not malleable: any change in its data
// should result in a different ID.
// Because our NewHeaderBody constructor enforces ParentView < View we use
// WithFieldGenerator to safely pass it.
func TestClusterBlockMalleability(t *testing.T) {
	clusterBlock := unittest.ClusterBlockFixture()
	unittest.RequireEntityNonMalleable(
		t,
		clusterBlock,
		unittest.WithFieldGenerator("HeaderBody.ParentView", func() uint64 {
			return clusterBlock.View - 1 // ParentView must stay below View, so set it to View-1
		}),
		unittest.WithFieldGenerator("Payload.Collection", func() flow.Collection {
			return unittest.CollectionFixture(3)
		}),
	)
}

// TestNewBlock verifies the behavior of the NewBlock constructor.
// It ensures proper handling of both valid and invalid untrusted input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UnsignedUntrustedBlock results in a valid UnsignedBlock.
//
// 2. Invalid input with invalid HeaderBody:
//   - Ensures an error is returned when the HeaderBody.ParentID is flow.ZeroID.
//
// 3. Invalid input with invalid Payload:
//   - Ensures an error is returned when the Payload contains a Collection with invalid transaction IDs.
func TestNewBlock(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		block := unittest.ClusterBlockFixture()

		res, err := cluster.NewBlock(cluster.UnsignedUntrustedBlock(*block))
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with invalid header body", func(t *testing.T) {
		block := unittest.ClusterBlockFixture()
		block.ParentID = flow.ZeroID

		res, err := cluster.NewBlock(cluster.UnsignedUntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid header body")
	})

	t.Run("invalid input with invalid payload", func(t *testing.T) {
		block := unittest.ClusterBlockFixture()
		collection := unittest.CollectionFixture(5)
		collection.Transactions[2] = nil
		block.Payload.Collection = collection

		res, err := cluster.NewBlock(cluster.UnsignedUntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid cluster payload")
	})
}

// TestNewRootBlock verifies the behavior of the NewRootBlock constructor.
// It ensures proper handling of both valid and invalid untrusted input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UnsignedUntrustedBlock results in a valid root UnsignedBlock.
//
// 2. Invalid input with invalid HeaderBody:
//   - Ensures an error is returned when the HeaderBody.ParentView is not zero.
//
// 3. Invalid input with invalid ParentID:
//   - Ensures an error is returned when the HeaderBody.ParentID is not zero.
//
// 4. Invalid input with invalid Payload:
//   - Ensures an error is returned when the Payload.ReferenceBlockID is not flow.ZeroID.
func TestNewRootBlock(t *testing.T) {
	// validRootBlockFixture returns a new valid root cluster.UntrustedUnsignedBlock for use in tests.
	validRootBlockFixture := func() cluster.UnsignedUntrustedBlock {
		return cluster.UnsignedUntrustedBlock{
			HeaderBody: flow.HeaderBody{
				ChainID:            flow.Emulator,
				ParentID:           flow.ZeroID,
				Height:             10,
				Timestamp:          uint64(time.Now().UnixMilli()),
				View:               0,
				ParentView:         0,
				ParentVoterIndices: []byte{},
				ParentVoterSigData: []byte{},
				ProposerID:         flow.ZeroID,
				LastViewTC:         nil,
			},
			Payload: *cluster.NewEmptyPayload(flow.ZeroID),
		}
	}

	t.Run("valid input", func(t *testing.T) {
		res, err := cluster.NewRootBlock(validRootBlockFixture())
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with invalid header body", func(t *testing.T) {
		block := validRootBlockFixture()
		block.ParentView = 1

		res, err := cluster.NewRootBlock(block)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid root header body")
	})

	t.Run("invalid input with invalid ParentID", func(t *testing.T) {
		block := validRootBlockFixture()
		block.ParentID = unittest.IdentifierFixture()

		res, err := cluster.NewRootBlock(block)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "ParentID must be zero")
	})

	t.Run("invalid input with invalid payload", func(t *testing.T) {
		block := validRootBlockFixture()
		block.Payload.ReferenceBlockID = unittest.IdentifierFixture()

		res, err := cluster.NewRootBlock(block)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid root cluster payload")
	})
}

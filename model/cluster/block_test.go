package cluster_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestClusterBlockMalleability checks that cluster.Block is not malleable: any change in its data
// should result in a different ID.
// Because our NewHeaderBody constructor enforces ParentView < View we use
// WithFieldGenerator to safely pass it.
func TestClusterBlockMalleability(t *testing.T) {

	clusterBlock := unittest.ClusterBlockFixture()
	unittest.RequireEntityNonMalleable(
		t,
		clusterBlock,
		unittest.WithFieldGenerator("Header.ParentView", func() uint64 {
			return clusterBlock.Header.View - 1 // ParentView must stay below View, so set it to View-1
		}),
		unittest.WithFieldGenerator("Header.Timestamp", func() time.Time { return time.Now().UTC() }),
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
//   - Verifies that a properly populated UntrustedBlock results in a valid Block.
//
// 2. Invalid input with invalid HeaderBody:
//   - Ensures an error is returned when the HeaderBody.ParentID is flow.ZeroID.
//
// 3. Invalid input with invalid Payload:
//   - Ensures an error is returned when the Payload contains a Collection with invalid transaction IDs.
func TestNewBlock(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		block := unittest.ClusterBlockFixture()

		res, err := cluster.NewBlock(cluster.UntrustedBlock(*block))
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with invalid header body", func(t *testing.T) {
		block := unittest.ClusterBlockFixture()
		block.Header.ParentID = flow.ZeroID

		res, err := cluster.NewBlock(cluster.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid header body")
	})

	t.Run("invalid input with invalid payload", func(t *testing.T) {
		block := unittest.ClusterBlockFixture()
		collection := unittest.CollectionFixture(5)
		collection.Transactions[2] = nil
		block.Payload.Collection = collection

		res, err := cluster.NewBlock(cluster.UntrustedBlock(*block))
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
//   - Verifies that a properly populated UntrustedBlock results in a valid root Block.
//
// 2. Invalid input with invalid HeaderBody:
//   - Ensures an error is returned when the HeaderBody.ParentView is not zero.
//
// 3. Invalid input with invalid Payload:
//   - Ensures an error is returned when the Payload.ReferenceBlockID is flow.ZeroID.
func TestNewRootBlock(t *testing.T) {
	base := cluster.UntrustedBlock{
		Header: flow.HeaderBody{
			ChainID:            flow.Emulator,
			ParentID:           flow.ZeroID,
			Height:             10,
			Timestamp:          time.Now(),
			View:               0,
			ParentView:         0,
			ParentVoterIndices: []byte{},
			ParentVoterSigData: []byte{},
			ProposerID:         flow.ZeroID,
			LastViewTC:         nil,
		},
		Payload: *cluster.NewEmptyPayload(flow.ZeroID),
	}

	t.Run("valid input", func(t *testing.T) {
		res, err := cluster.NewRootBlock(base)
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with invalid header body", func(t *testing.T) {
		block := base
		block.Header.ParentView = 1

		res, err := cluster.NewRootBlock(block)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid root header body")
	})

	t.Run("invalid input with invalid payload", func(t *testing.T) {
		block := base
		block.Payload.ReferenceBlockID = unittest.IdentifierFixture()

		res, err := cluster.NewRootBlock(block)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid root cluster payload")
	})
}

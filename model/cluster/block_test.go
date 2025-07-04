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
func TestClusterBlockMalleability(t *testing.T) {
	unittest.RequireEntityNonMalleable(
		t,
		unittest.ClusterBlockFixture(),
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
// 2. Invalid input with zero ParentID:
//   - Ensures an error is returned when the Header.ParentID is flow.ZeroID.
//
// 3. Invalid input with nil ParentVoterIndices:
//   - Ensures an error is returned when the Header.ParentVoterIndices is nil.
//
// 4. Invalid input with empty ParentVoterIndices:
//   - Ensures an error is returned when the Header.ParentVoterIndices is an empty slice.
//
// 5. Invalid input with nil ParentVoterSigData:
//   - Ensures an error is returned when the Header.ParentVoterSigData is nil.
//
// 6. Invalid input with empty ParentVoterSigData:
//   - Ensures an error is returned when the Header.ParentVoterSigData is an empty slice.
//
// 7. Invalid input with zero ProposerID:
//   - Ensures an error is returned when the Header.ProposerID is flow.ZeroID.
//
// 8. Invalid input where ParentView is greater than or equal to View:
//   - Ensures an error is returned when the Header.ParentView is not less than Header.View.
//
// 9. Invalid input with malformed Collection in payload:
//   - Ensures an error is returned when the Payload contains a Collection with invalid transaction IDs.
func TestNewBlock(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		block := unittest.ClusterBlockFixture()

		res, err := cluster.NewBlock(cluster.UntrustedBlock(*block))
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with zero ParentID", func(t *testing.T) {
		block := unittest.ClusterBlockFixture()
		block.Header.ParentID = flow.ZeroID

		res, err := cluster.NewBlock(cluster.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "parent ID must not be zero")
	})

	t.Run("invalid input with nil ParentVoterIndices", func(t *testing.T) {
		block := unittest.ClusterBlockFixture()
		block.Header.ParentVoterIndices = nil

		res, err := cluster.NewBlock(cluster.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "parent voter indices must not be empty")
	})

	t.Run("invalid input with empty ParentVoterIndices", func(t *testing.T) {
		block := unittest.ClusterBlockFixture()
		block.Header.ParentVoterIndices = []byte{}

		res, err := cluster.NewBlock(cluster.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "parent voter indices must not be empty")
	})

	t.Run("invalid input with nil ParentVoterSigData", func(t *testing.T) {
		block := unittest.ClusterBlockFixture()
		block.Header.ParentVoterSigData = nil

		res, err := cluster.NewBlock(cluster.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "parent voter signature must not be empty")
	})

	t.Run("invalid input with empty ParentVoterSigData", func(t *testing.T) {
		block := unittest.ClusterBlockFixture()
		block.Header.ParentVoterSigData = []byte{}

		res, err := cluster.NewBlock(cluster.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "parent voter signature must not be empty")
	})

	t.Run("invalid input with zero ProposerID", func(t *testing.T) {
		block := unittest.ClusterBlockFixture()
		block.Header.ProposerID = flow.ZeroID

		res, err := cluster.NewBlock(cluster.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "proposer ID must not be zero")
	})

	t.Run("invalid input with ParentView is greater than or equal to View", func(t *testing.T) {
		block := unittest.ClusterBlockFixture()
		block.Header.ParentView = 10
		block.Header.View = 10

		res, err := cluster.NewBlock(cluster.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid views - block view (10) ends after the parent block view (10)")
	})

	t.Run("invalid input with malformed Collection in payload", func(t *testing.T) {
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

package inmem_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/state/protocol"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestFromSnapshot tests that we are able to convert a database-backed snapshot
// to a memory-backed snapshot.
func TestFromSnapshot(t *testing.T) {
	identities := unittest.IdentityListFixture(10, unittest.WithAllRoles())
	rootSnapshot := unittest.RootSnapshotFixture(identities)

	util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {

		epochBuilder := unittest.NewEpochBuilder(t, state)
		// build epoch 1 (prepare epoch 2)
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()
		// build epoch 2 (prepare epoch 3)
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()

		// get heights of each phase in built epochs
		epoch1, ok := epochBuilder.EpochHeights(1)
		require.True(t, ok)
		epoch2, ok := epochBuilder.EpochHeights(2)
		require.True(t, ok)

		// test that we are able retrieve an in-memory version of root snapshot
		t.Run("root snapshot", func(t *testing.T) {
			root, err := state.Params().Root()
			require.NoError(t, err)
			expected := state.AtHeight(root.Height)
			actual, err := inmem.FromSnapshot(expected)
			require.NoError(t, err)
			assertSnapshotsEqual(t, expected, actual)
			testEncodeDecode(t, actual)
		})

		// test getting an in-memory snapshot for all phase of epoch 1
		t.Run("epoch 1", func(t *testing.T) {
			t.Run("staking phase", func(t *testing.T) {
				expected := state.AtHeight(epoch1.Staking)
				actual, err := inmem.FromSnapshot(expected)
				require.NoError(t, err)
				assertSnapshotsEqual(t, expected, actual)
				testEncodeDecode(t, actual)
			})
			t.Run("setup phase", func(t *testing.T) {
				expected := state.AtHeight(epoch1.Setup)
				actual, err := inmem.FromSnapshot(expected)
				require.NoError(t, err)
				assertSnapshotsEqual(t, expected, actual)
				testEncodeDecode(t, actual)
			})
			t.Run("committed phase", func(t *testing.T) {
				expected := state.AtHeight(epoch1.Committed)
				actual, err := inmem.FromSnapshot(expected)
				require.NoError(t, err)
				assertSnapshotsEqual(t, expected, actual)
				testEncodeDecode(t, actual)
			})
		})

		// test getting an in-memory snapshot for all phase of epoch 2
		t.Run("epoch 2", func(t *testing.T) {
			t.Run("staking phase", func(t *testing.T) {
				expected := state.AtHeight(epoch2.Staking)
				actual, err := inmem.FromSnapshot(expected)
				require.NoError(t, err)
				assertSnapshotsEqual(t, expected, actual)
				testEncodeDecode(t, actual)
			})
			t.Run("setup phase", func(t *testing.T) {
				expected := state.AtHeight(epoch2.Setup)
				actual, err := inmem.FromSnapshot(expected)
				require.NoError(t, err)
				assertSnapshotsEqual(t, expected, actual)
				testEncodeDecode(t, actual)
			})
			t.Run("committed phase", func(t *testing.T) {
				expected := state.AtHeight(epoch2.Committed)
				actual, err := inmem.FromSnapshot(expected)
				require.NoError(t, err)
				assertSnapshotsEqual(t, expected, actual)
				testEncodeDecode(t, actual)
			})
		})
	})
}

// checks that a snapshot is equivalent after encoding and decoding
func testEncodeDecode(t *testing.T, snap *inmem.Snapshot) {

	bz, err := json.Marshal(snap.Encodable())
	require.NoError(t, err)

	var encoded inmem.EncodableSnapshot
	err = json.Unmarshal(bz, &encoded)
	require.NoError(t, err)

	fromEncoded := inmem.SnapshotFromEncodable(encoded)
	assertSnapshotsEqual(t, snap, fromEncoded)
}

// checks that 2 snapshots are equivalent by converting to a serializable
// representation and comparing the serializations
func snapshotsEqual(t *testing.T, snap1, snap2 protocol.Snapshot) bool {
	enc1, err := inmem.FromSnapshot(snap1)
	require.NoError(t, err)
	enc2, err := inmem.FromSnapshot(snap2)
	require.NoError(t, err)

	bz1, err := json.Marshal(enc1.Encodable())
	require.NoError(t, err)
	bz2, err := json.Marshal(enc2.Encodable())
	require.NoError(t, err)

	return bytes.Equal(bz1, bz2)
}

func assertSnapshotsEqual(t *testing.T, snap1, snap2 protocol.Snapshot) {
	assert.True(t, snapshotsEqual(t, snap1, snap2))
}

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

		// Prepare an epoch builder, which builds epochs with 4 blocks, A,B,C,D
		// See EpochBuilder documentation for details of these blocks.
		//
		epochBuilder := unittest.NewEpochBuilder(t, state)
		// build blocks WITHIN epoch 1 - PREPARING epoch 2
		// A - height 0 (root block)
		// B - height 1 - staking phase
		// C - height 2 - setup phase
		// D - height 3 - committed phase
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()
		// build blocks WITHIN epoch 2 - PREPARING epoch 3
		// A - height 4
		// B - height 5 - staking phase
		// C - height 6 - setup phase
		// D - height 7 - committed phase
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()

		// test that we are able retrieve an in-memory version of root snapshot
		t.Run("root snapshot", func(t *testing.T) {
			expected := state.AtHeight(0)
			actual, err := inmem.FromSnapshot(expected)
			require.NoError(t, err)
			assertSnapshotsEqual(t, expected, actual)
			testEncodeDecode(t, actual)
		})

		// test getting an in-memory snapshot for all phase of epoch 1
		t.Run("epoch 1", func(t *testing.T) {
			t.Run("staking phase", func(t *testing.T) {
				expected := state.AtHeight(1)
				actual, err := inmem.FromSnapshot(expected)
				require.NoError(t, err)
				assertSnapshotsEqual(t, expected, actual)
				testEncodeDecode(t, actual)
			})
			t.Run("setup phase", func(t *testing.T) {
				expected := state.AtHeight(2)
				actual, err := inmem.FromSnapshot(expected)
				require.NoError(t, err)
				assertSnapshotsEqual(t, expected, actual)
				testEncodeDecode(t, actual)
			})
			t.Run("committed phase", func(t *testing.T) {
				expected := state.AtHeight(3)
				actual, err := inmem.FromSnapshot(expected)
				require.NoError(t, err)
				assertSnapshotsEqual(t, expected, actual)
				testEncodeDecode(t, actual)
			})
		})

		// test getting an in-memory snapshot for all phase of epoch 2
		t.Run("epoch 2", func(t *testing.T) {
			t.Run("staking phase", func(t *testing.T) {
				expected := state.AtHeight(5)
				actual, err := inmem.FromSnapshot(expected)
				require.NoError(t, err)
				assertSnapshotsEqual(t, expected, actual)
				testEncodeDecode(t, actual)
			})
			t.Run("setup phase", func(t *testing.T) {
				expected := state.AtHeight(6)
				actual, err := inmem.FromSnapshot(expected)
				require.NoError(t, err)
				assertSnapshotsEqual(t, expected, actual)
				testEncodeDecode(t, actual)
			})
			t.Run("committed phase", func(t *testing.T) {
				expected := state.AtHeight(7)
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

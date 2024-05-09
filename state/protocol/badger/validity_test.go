package badger

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

var participants = unittest.IdentityListFixture(20, unittest.WithAllRoles())

// TestEntityExpirySnapshotValidation tests that we perform correct sanity checks when
// bootstrapping consensus nodes and access nodes we expect that we only bootstrap snapshots
// with sufficient history.
func TestEntityExpirySnapshotValidation(t *testing.T) {
	t.Run("spork-root-snapshot", func(t *testing.T) {
		rootSnapshot := unittest.RootSnapshotFixture(participants)
		err := ValidRootSnapshotContainsEntityExpiryRange(rootSnapshot)
		require.NoError(t, err)
	})
	t.Run("not-enough-history", func(t *testing.T) {
		rootSnapshot := unittest.RootSnapshotFixture(participants)
		rootSnapshot.Encodable().Head.Height += 10 // advance height to be not spork root snapshot
		err := ValidRootSnapshotContainsEntityExpiryRange(rootSnapshot)
		require.Error(t, err)
	})
	t.Run("enough-history-spork-just-started", func(t *testing.T) {
		rootSnapshot := unittest.RootSnapshotFixture(participants)
		// advance height to be not spork root snapshot, but still lower than transaction expiry
		rootSnapshot.Encodable().Head.Height += flow.DefaultTransactionExpiry / 2
		// add blocks to sealing segment
		rootSnapshot.Encodable().SealingSegment.ExtraBlocks = unittest.BlockFixtures(int(flow.DefaultTransactionExpiry / 2))
		err := ValidRootSnapshotContainsEntityExpiryRange(rootSnapshot)
		require.NoError(t, err)
	})
	t.Run("enough-history-long-spork", func(t *testing.T) {
		rootSnapshot := unittest.RootSnapshotFixture(participants)
		// advance height to be not spork root snapshot
		rootSnapshot.Encodable().Head.Height += flow.DefaultTransactionExpiry * 2
		// add blocks to sealing segment
		rootSnapshot.Encodable().SealingSegment.ExtraBlocks = unittest.BlockFixtures(int(flow.DefaultTransactionExpiry) - 1)
		err := ValidRootSnapshotContainsEntityExpiryRange(rootSnapshot)
		require.NoError(t, err)
	})
	t.Run("more-history-than-needed", func(t *testing.T) {
		rootSnapshot := unittest.RootSnapshotFixture(participants)
		// advance height to be not spork root snapshot
		rootSnapshot.Encodable().Head.Height += flow.DefaultTransactionExpiry * 2
		// add blocks to sealing segment
		rootSnapshot.Encodable().SealingSegment.ExtraBlocks = unittest.BlockFixtures(flow.DefaultTransactionExpiry * 2)
		err := ValidRootSnapshotContainsEntityExpiryRange(rootSnapshot)
		require.NoError(t, err)
	})
}

func TestValidateVersionBeacon(t *testing.T) {
	t.Run("no version beacon is ok", func(t *testing.T) {
		snap := new(mock.Snapshot)

		snap.On("VersionBeacon").Return(nil, nil)

		err := validateVersionBeacon(snap)
		require.NoError(t, err)
	})
	t.Run("valid version beacon is ok", func(t *testing.T) {
		snap := new(mock.Snapshot)
		block := unittest.BlockFixture()
		block.Header.Height = 100

		vb := &flow.SealedVersionBeacon{
			VersionBeacon: &flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{
						BlockHeight: 1000,
						Version:     "1.0.0",
					},
				},
				Sequence: 50,
			},
			SealHeight: uint64(37),
		}

		snap.On("Head").Return(block.Header, nil)
		snap.On("VersionBeacon").Return(vb, nil)

		err := validateVersionBeacon(snap)
		require.NoError(t, err)
	})
	t.Run("height must be below highest block", func(t *testing.T) {
		snap := new(mock.Snapshot)
		block := unittest.BlockFixture()
		block.Header.Height = 12

		vb := &flow.SealedVersionBeacon{
			VersionBeacon: &flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{
						BlockHeight: 1000,
						Version:     "1.0.0",
					},
				},
				Sequence: 50,
			},
			SealHeight: uint64(37),
		}

		snap.On("Head").Return(block.Header, nil)
		snap.On("VersionBeacon").Return(vb, nil)

		err := validateVersionBeacon(snap)
		require.Error(t, err)
		require.True(t, protocol.IsInvalidServiceEventError(err))
	})
	t.Run("version beacon must be valid", func(t *testing.T) {
		snap := new(mock.Snapshot)
		block := unittest.BlockFixture()
		block.Header.Height = 12

		vb := &flow.SealedVersionBeacon{
			VersionBeacon: &flow.VersionBeacon{
				VersionBoundaries: []flow.VersionBoundary{
					{
						BlockHeight: 0,
						Version:     "asdf", // invalid semver - hence will be considered invalid
					},
				},
				Sequence: 50,
			},
			SealHeight: uint64(1),
		}

		snap.On("Head").Return(block.Header, nil)
		snap.On("VersionBeacon").Return(vb, nil)

		err := validateVersionBeacon(snap)
		require.Error(t, err)
		require.True(t, protocol.IsInvalidServiceEventError(err))
	})
}

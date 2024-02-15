package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestResults_IndexByServiceEvents(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		height1 := uint64(21)
		height2 := uint64(37)
		height3 := uint64(55)
		vb1 := flow.SealedVersionBeacon{
			VersionBeacon: unittest.VersionBeaconFixture(
				unittest.WithBoundaries(
					flow.VersionBoundary{
						Version:     "1.0.0",
						BlockHeight: height1 + 5,
					},
				),
			),
			SealHeight: height1,
		}
		vb2 := flow.SealedVersionBeacon{
			VersionBeacon: unittest.VersionBeaconFixture(
				unittest.WithBoundaries(
					flow.VersionBoundary{
						Version:     "1.1.0",
						BlockHeight: height2 + 5,
					},
				),
			),
			SealHeight: height2,
		}
		vb3 := flow.SealedVersionBeacon{
			VersionBeacon: unittest.VersionBeaconFixture(
				unittest.WithBoundaries(
					flow.VersionBoundary{
						Version:     "2.0.0",
						BlockHeight: height3 + 5,
					},
				),
			),
			SealHeight: height3,
		}

		// indexing 3 version beacons at different heights
		err := db.Update(IndexVersionBeaconByHeight(&vb1))
		require.NoError(t, err)

		err = db.Update(IndexVersionBeaconByHeight(&vb2))
		require.NoError(t, err)

		err = db.Update(IndexVersionBeaconByHeight(&vb3))
		require.NoError(t, err)

		// index version beacon 2 again to make sure we tolerate duplicates
		// it is possible for two or more events of the same type to be from the same height
		err = db.Update(IndexVersionBeaconByHeight(&vb2))
		require.NoError(t, err)

		t.Run("retrieve exact height match", func(t *testing.T) {
			var actualVB flow.SealedVersionBeacon
			err := db.View(LookupLastVersionBeaconByHeight(height1, &actualVB))
			require.NoError(t, err)
			require.Equal(t, vb1, actualVB)

			err = db.View(LookupLastVersionBeaconByHeight(height2, &actualVB))
			require.NoError(t, err)
			require.Equal(t, vb2, actualVB)

			err = db.View(LookupLastVersionBeaconByHeight(height3, &actualVB))
			require.NoError(t, err)
			require.Equal(t, vb3, actualVB)
		})

		t.Run("finds highest but not higher than given", func(t *testing.T) {
			var actualVB flow.SealedVersionBeacon

			err := db.View(LookupLastVersionBeaconByHeight(height3-1, &actualVB))
			require.NoError(t, err)
			require.Equal(t, vb2, actualVB)
		})

		t.Run("finds highest", func(t *testing.T) {
			var actualVB flow.SealedVersionBeacon

			err := db.View(LookupLastVersionBeaconByHeight(height3+1, &actualVB))
			require.NoError(t, err)
			require.Equal(t, vb3, actualVB)
		})

		t.Run("height below lowest entry returns nothing", func(t *testing.T) {
			var actualVB flow.SealedVersionBeacon

			err := db.View(LookupLastVersionBeaconByHeight(height1-1, &actualVB))
			require.ErrorIs(t, err, storage.ErrNotFound)
		})
	})
}

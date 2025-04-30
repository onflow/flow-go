package operation_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestResults_IndexByServiceEvents(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
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
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexVersionBeaconByHeight(rw.Writer(), &vb1)
		}))

		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexVersionBeaconByHeight(rw.Writer(), &vb2)
		}))

		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexVersionBeaconByHeight(rw.Writer(), &vb3)
		}))

		// index version beacon 2 again to make sure we tolerate duplicates
		// it is possible for two or more events of the same type to be from the same height
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexVersionBeaconByHeight(rw.Writer(), &vb2)
		}))

		t.Run("retrieve exact height match", func(t *testing.T) {
			var actualVB flow.SealedVersionBeacon
			err := operation.LookupLastVersionBeaconByHeight(db.Reader(), height1, &actualVB)
			require.NoError(t, err)
			require.Equal(t, vb1, actualVB)

			err = operation.LookupLastVersionBeaconByHeight(db.Reader(), height2, &actualVB)
			require.NoError(t, err)
			require.Equal(t, vb2, actualVB)

			err = operation.LookupLastVersionBeaconByHeight(db.Reader(), height3, &actualVB)
			require.NoError(t, err)
			require.Equal(t, vb3, actualVB)
		})

		t.Run("finds highest but not higher than given", func(t *testing.T) {
			var actualVB flow.SealedVersionBeacon

			err := operation.LookupLastVersionBeaconByHeight(db.Reader(), height3-1, &actualVB)
			require.NoError(t, err)
			require.Equal(t, vb2, actualVB)
		})

		t.Run("finds highest", func(t *testing.T) {
			var actualVB flow.SealedVersionBeacon

			err := operation.LookupLastVersionBeaconByHeight(db.Reader(), height3+1, &actualVB)
			require.NoError(t, err)
			require.Equal(t, vb3, actualVB)
		})

		t.Run("height below lowest entry returns nothing", func(t *testing.T) {
			var actualVB flow.SealedVersionBeacon

			err := operation.LookupLastVersionBeaconByHeight(db.Reader(), height1-1, &actualVB)
			require.ErrorIs(t, err, storage.ErrNotFound)
		})
	})
}

package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func TestResults_IndexByServiceEvents(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		vb1 := unittest.VersionBeaconFixture(2)
		vb2 := unittest.VersionBeaconFixture(2)
		vb3 := unittest.VersionBeaconFixture(2)
		height1 := uint64(21)
		height2 := uint64(37)
		height3 := uint64(55)

		// inserting 3 results at different height each has a ServiceEventCommit
		err := db.Update(IndexVersionBeaconByHeight(vb1, height1))
		require.NoError(t, err)

		err = db.Update(IndexVersionBeaconByHeight(vb2, height2))
		require.NoError(t, err)

		err = db.Update(IndexVersionBeaconByHeight(vb3, height3))
		require.NoError(t, err)

		// insert result 2 again to make sure we tolerate duplicates
		// it is possible for two or more events of the same type to be from the same height
		err = db.Update(IndexVersionBeaconByHeight(vb2, height2))
		require.NoError(t, err)

		t.Run("retrieve exact height match", func(t *testing.T) {
			var actualVB flow.VersionBeacon
			var actualHeight uint64
			err := db.View(LookupLastVersionBeaconByHeight(height1, &actualVB, &actualHeight))
			require.NoError(t, err)
			require.Equal(t, *vb1, actualVB)

			err = db.View(LookupLastVersionBeaconByHeight(height2, &actualVB, &actualHeight))
			require.NoError(t, err)
			require.Equal(t, *vb2, actualVB)

			err = db.View(LookupLastVersionBeaconByHeight(height3, &actualVB, &actualHeight))
			require.NoError(t, err)
			require.Equal(t, *vb3, actualVB)
		})

		t.Run("finds highest but not higher than given", func(t *testing.T) {
			var actualVB flow.VersionBeacon
			var actualHeight uint64

			err := db.View(LookupLastVersionBeaconByHeight(height3-1, &actualVB, &actualHeight))
			require.NoError(t, err)
			require.Equal(t, *vb2, actualVB)
		})

		t.Run("finds highest", func(t *testing.T) {
			var actualVB flow.VersionBeacon
			var actualHeight uint64

			err := db.View(LookupLastVersionBeaconByHeight(height3+1, &actualVB, &actualHeight))
			require.NoError(t, err)
			require.Equal(t, *vb3, actualVB)
		})

		t.Run("height below lowest entry returns nothing", func(t *testing.T) {
			var actualVB flow.VersionBeacon
			var actualHeight uint64

			err := db.View(LookupLastVersionBeaconByHeight(height1-1, &actualVB, &actualHeight))
			require.ErrorIs(t, err, storage.ErrNotFound)
		})

	})
}

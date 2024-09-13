package operation

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestFinalizedInsertUpdateRetrieve(t *testing.T) {
	unittest.RunWithWrappedPebbleDB(t, func(db *unittest.PebbleWrapper) {
		height := uint64(1337)

		err := db.Update(InsertFinalizedHeight(height))
		require.NoError(t, err)

		var retrieved uint64
		err = db.View(RetrieveFinalizedHeight(&retrieved))
		require.NoError(t, err)

		assert.Equal(t, retrieved, height)

		height = 9999
		err = db.Update(UpdateFinalizedHeight(height))
		require.NoError(t, err)

		err = db.View(RetrieveFinalizedHeight(&retrieved))
		require.NoError(t, err)

		assert.Equal(t, retrieved, height)
	})
}

func TestSealedInsertUpdateRetrieve(t *testing.T) {
	unittest.RunWithWrappedPebbleDB(t, func(db *unittest.PebbleWrapper) {
		height := uint64(1337)

		err := db.Update(InsertSealedHeight(height))
		require.NoError(t, err)

		var retrieved uint64
		err = db.View(RetrieveSealedHeight(&retrieved))
		require.NoError(t, err)

		assert.Equal(t, retrieved, height)

		height = 9999
		err = db.Update(UpdateSealedHeight(height))
		require.NoError(t, err)

		err = db.View(RetrieveSealedHeight(&retrieved))
		require.NoError(t, err)

		assert.Equal(t, retrieved, height)
	})
}

func TestEpochFirstBlockIndex_InsertRetrieve(t *testing.T) {
	unittest.RunWithWrappedPebbleDB(t, func(db *unittest.PebbleWrapper) {
		height := rand.Uint64()
		epoch := rand.Uint64()

		// retrieve when empty errors
		var retrieved uint64
		err := db.View(RetrieveEpochFirstHeight(epoch, &retrieved))
		require.ErrorIs(t, err, storage.ErrNotFound)

		// can insert
		err = db.Update(InsertEpochFirstHeight(epoch, height))
		require.NoError(t, err)

		// can retrieve
		err = db.View(RetrieveEpochFirstHeight(epoch, &retrieved))
		require.NoError(t, err)
		assert.Equal(t, retrieved, height)

		// retrieve non-existent key errors
		err = db.View(RetrieveEpochFirstHeight(epoch+1, &retrieved))
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestLastCompleteBlockHeightInsertUpdateRetrieve(t *testing.T) {
	unittest.RunWithWrappedPebbleDB(t, func(db *unittest.PebbleWrapper) {
		height := uint64(1337)

		err := db.Update(InsertLastCompleteBlockHeight(height))
		require.NoError(t, err)

		var retrieved uint64
		err = db.View(RetrieveLastCompleteBlockHeight(&retrieved))
		require.NoError(t, err)

		assert.Equal(t, retrieved, height)

		height = 9999
		err = db.Update(UpdateLastCompleteBlockHeight(height))
		require.NoError(t, err)

		err = db.View(RetrieveLastCompleteBlockHeight(&retrieved))
		require.NoError(t, err)

		assert.Equal(t, retrieved, height)
	})
}

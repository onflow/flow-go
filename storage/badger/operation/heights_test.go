package operation

import (
	"math/rand"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestFinalizedInsertUpdateRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
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
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
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
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
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

		// insert existent key errors
		err = db.Update(InsertEpochFirstHeight(epoch, height))
		require.ErrorIs(t, err, storage.ErrAlreadyExists)
	})
}

func TestLastCompleteBlockHeightInsertUpdateRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
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

func TestLastCompleteBlockHeightInsertIfNotExists(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		height1 := uint64(1337)

		err := db.Update(InsertLastCompleteBlockHeightIfNotExists(height1))
		require.NoError(t, err)

		var retrieved uint64
		err = db.View(RetrieveLastCompleteBlockHeight(&retrieved))
		require.NoError(t, err)

		assert.Equal(t, retrieved, height1)

		height2 := uint64(9999)
		err = db.Update(InsertLastCompleteBlockHeightIfNotExists(height2))
		require.NoError(t, err)

		err = db.View(RetrieveLastCompleteBlockHeight(&retrieved))
		require.NoError(t, err)

		assert.Equal(t, retrieved, height1)
	})
}

package operation

import (
	"math/rand"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestInsertMyDKGPrivateInfo_StoreRetrieve tests writing and reading private DKG info.
func TestMyDKGPrivateInfo_StoreRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		t.Run("when not stored", func(t *testing.T) {
			var stored dkg.DKGParticipantPriv
			err := db.View(RetrieveMyDKGPrivateInfo(1, &stored))
			assert.ErrorIs(t, err, storage.ErrNotFound)
		})

		t.Run("should be able to store and read", func(t *testing.T) {
			epochCounter := rand.Uint64()
			info := unittest.DKGParticipantPriv()

			// should be able to store
			err := db.Update(InsertMyDKGPrivateInfo(epochCounter, info))
			assert.NoError(t, err)

			// should be able to read
			var stored dkg.DKGParticipantPriv
			err = db.View(RetrieveMyDKGPrivateInfo(epochCounter, &stored))
			assert.NoError(t, err)
			assert.Equal(t, info, &stored)

			// should fail to read other epoch counter
			err = db.View(RetrieveMyDKGPrivateInfo(rand.Uint64(), &stored))
			assert.ErrorIs(t, err, storage.ErrNotFound)
		})
	})
}

// TestDKGStartedForEpoch tests setting the DKG-started flag.
func TestDKGStartedForEpoch(t *testing.T) {

	t.Run("reading when unset should return false", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			var started bool
			err := db.View(RetrieveDKGStartedForEpoch(1, &started))
			assert.NoError(t, err)
			assert.False(t, started)
		})
	})

	t.Run("should be able to set flag to true", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			epochCounter := rand.Uint64()

			// set the flag, ensure no error
			err := db.Update(InsertDKGStartedForEpoch(epochCounter))
			assert.NoError(t, err)

			// read the flag, should be true now
			var started bool
			err = db.View(RetrieveDKGStartedForEpoch(epochCounter, &started))
			assert.NoError(t, err)
			assert.True(t, started)

			// read the flag for a different epoch, should be false
			err = db.View(RetrieveDKGStartedForEpoch(epochCounter+1, &started))
			assert.NoError(t, err)
			assert.False(t, started)
		})
	})
}

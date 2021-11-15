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

func TestInsertMyDKGPrivateInfo_StoreRetrieve(t *testing.T) {
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

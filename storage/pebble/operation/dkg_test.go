package operation

import (
	"math/rand"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestInsertMyDKGPrivateInfo_StoreRetrieve tests writing and reading private DKG info.
func TestMyBeaconPrivateKey_StoreRetrieve(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

		t.Run("should return error not found when not stored", func(t *testing.T) {
			var stored encodable.RandomBeaconPrivKey
			err := RetrieveMyBeaconPrivateKey(1, &stored)(db)
			assert.ErrorIs(t, err, storage.ErrNotFound)
		})

		t.Run("should be able to store and read", func(t *testing.T) {
			epochCounter := rand.Uint64()
			info := unittest.RandomBeaconPriv()

			// should be able to store
			err := InsertMyBeaconPrivateKey(epochCounter, info)(db)
			assert.NoError(t, err)

			// should be able to read
			var stored encodable.RandomBeaconPrivKey
			err = RetrieveMyBeaconPrivateKey(epochCounter, &stored)(db)
			assert.NoError(t, err)
			assert.Equal(t, info, &stored)

			// should fail to read other epoch counter
			err = RetrieveMyBeaconPrivateKey(rand.Uint64(), &stored)(db)
			assert.ErrorIs(t, err, storage.ErrNotFound)
		})
	})
}

// TestDKGStartedForEpoch tests setting the DKG-started flag.
func TestDKGStartedForEpoch(t *testing.T) {

	t.Run("reading when unset should return false", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			var started bool
			err := RetrieveDKGStartedForEpoch(1, &started)(db)
			assert.NoError(t, err)
			assert.False(t, started)
		})
	})

	t.Run("should be able to set flag to true", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			epochCounter := rand.Uint64()

			// set the flag, ensure no error
			err := InsertDKGStartedForEpoch(epochCounter)(db)
			assert.NoError(t, err)

			// read the flag, should be true now
			var started bool
			err = RetrieveDKGStartedForEpoch(epochCounter, &started)(db)
			assert.NoError(t, err)
			assert.True(t, started)

			// read the flag for a different epoch, should be false
			err = RetrieveDKGStartedForEpoch(epochCounter+1, &started)(db)
			assert.NoError(t, err)
			assert.False(t, started)
		})
	})
}

func TestDKGEndStateForEpoch(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		epochCounter := rand.Uint64()

		// should be able to write end state
		endState := flow.DKGEndStateSuccess
		err := InsertDKGEndStateForEpoch(epochCounter, endState)(db)
		assert.NoError(t, err)

		// should be able to read end state
		var readEndState flow.DKGEndState
		err = RetrieveDKGEndStateForEpoch(epochCounter, &readEndState)(db)
		assert.NoError(t, err)
		assert.Equal(t, endState, readEndState)

		// attempting to overwrite should error
		err = InsertDKGEndStateForEpoch(epochCounter, flow.DKGEndStateDKGFailure)(db)
		assert.ErrorIs(t, err, storage.ErrAlreadyExists)
	})
}

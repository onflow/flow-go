package operation

import (
	"encoding/binary"
	"math/rand"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestInsertMyDKGPrivateInfo_StoreRetrieve tests writing and reading private DKG info.
func TestMyBeaconPrivateKey_StoreRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		t.Run("should return error not found when not stored", func(t *testing.T) {
			var stored encodable.RandomBeaconPrivKey
			err := db.View(RetrieveMyBeaconPrivateKey(1, &stored))
			assert.ErrorIs(t, err, storage.ErrNotFound)
		})

		t.Run("should be able to store and read", func(t *testing.T) {
			epochCounter := rand.Uint64()
			info := unittest.RandomBeaconPriv()

			// should be able to store
			err := db.Update(InsertMyBeaconPrivateKey(epochCounter, info))
			assert.NoError(t, err)

			// should be able to read
			var stored encodable.RandomBeaconPrivKey
			err = db.View(RetrieveMyBeaconPrivateKey(epochCounter, &stored))
			assert.NoError(t, err)
			assert.Equal(t, info, &stored)

			// should fail to read other epoch counter
			err = db.View(RetrieveMyBeaconPrivateKey(rand.Uint64(), &stored))
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
			err := db.Update(UpsertDKGStateForEpoch(epochCounter, flow.DKGStateStarted))
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

func TestDKGSetStateForEpoch(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		epochCounter := rand.Uint64()

		// should be able to write new state
		newState := flow.DKGStateStarted
		err := db.Update(UpsertDKGStateForEpoch(epochCounter, newState))
		assert.NoError(t, err)

		// should be able to read current state
		var readCurrentState flow.DKGState
		err = db.View(RetrieveDKGStateForEpoch(epochCounter, &readCurrentState))
		assert.NoError(t, err)
		assert.Equal(t, newState, readCurrentState)

		// attempting to overwrite should succeed
		err = db.Update(UpsertDKGStateForEpoch(epochCounter, flow.DKGStateFailure))
		assert.NoError(t, err)
	})
}

// TestMigrateDKGEndStateFromV1 tests the migration of DKG end states from v1 to v2.
// All possible states in v1 are generated and then checked against the expected states in v2.
// Afterward the states are then migrated we check that old key was indeed removed and new key was added.
// This test also checks that the migration is idempotent after the first run.
func TestMigrateDKGEndStateFromV1(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		epochCounter := rand.Uint64() % 100

		preMigrationStates := make(map[uint64]uint32)
		for i := epochCounter; i < epochCounter+100; i++ {
			state := rand.Uint32() % 5 // [0,4] were supported in v1
			err := db.Update(insert(makePrefix(codeDKGEndState, i), state))
			assert.NoError(t, err)
			preMigrationStates[i] = state
		}

		assertExpectedState := func(oldState uint32, newState flow.DKGState) {
			switch oldState {
			case 0: // DKGEndStateUnknown
				assert.Equal(t, flow.DKGStateUninitialized, newState)
			case 1: // DKGEndStateSuccess
				assert.Equal(t, flow.RandomBeaconKeyCommitted, newState)
			case 2, 3, 4: // DKGEndStateInconsistentKey, DKGEndStateNoKey, DKGEndStateDKGFailure
				assert.Equal(t, flow.DKGStateFailure, newState)
			default:
				assert.Fail(t, "unexpected state")
			}
		}

		// migrate the state
		err := db.Update(MigrateDKGEndStateFromV1(zerolog.Nop()))
		assert.NoError(t, err)

		assertMigrationSuccessful := func() {
			// ensure previous keys were removed
			err = db.View(traverse(makePrefix(codeDKGEndState), func() (checkFunc, createFunc, handleFunc) {
				assert.Fail(t, "no keys should have been found")
				return nil, nil, nil
			}))
			assert.NoError(t, err)

			migratedStates := make(map[uint64]flow.DKGState)
			err = db.View(traverse(makePrefix(codeDKGState), func() (checkFunc, createFunc, handleFunc) {
				var epochCounter uint64
				check := func(key []byte) bool {
					epochCounter = binary.BigEndian.Uint64(key[1:]) // omit code
					return true
				}
				var newState flow.DKGState
				create := func() interface{} {
					return &newState
				}
				handle := func() error {
					migratedStates[epochCounter] = newState
					return nil
				}
				return check, create, handle
			}))
			assert.NoError(t, err)
			assert.Equal(t, len(preMigrationStates), len(migratedStates))
			for epochCounter, newState := range migratedStates {
				assertExpectedState(preMigrationStates[epochCounter], newState)
			}
		}
		assertMigrationSuccessful()

		// migrating again should be no-op
		err = db.Update(MigrateDKGEndStateFromV1(zerolog.Nop()))
		assert.NoError(t, err)
		assertMigrationSuccessful()
	})
}

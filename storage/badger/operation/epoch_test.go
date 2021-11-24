package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEpochEmergencyFallback(t *testing.T) {

	// the block ID where EECC was triggered
	blockID := unittest.IdentifierFixture()

	t.Run("reading when unset should return false", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			var triggered bool
			err := db.View(CheckEpochEmergencyFallbackTriggered(&triggered))
			assert.NoError(t, err)
			assert.False(t, triggered)
		})
	})
	t.Run("should be able to set flag to true", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			// set the flag, ensure no error
			err := db.Update(SetEpochEmergencyFallbackTriggered(blockID))
			assert.NoError(t, err)

			// read the flag, should be true now
			var triggered bool
			err = db.View(CheckEpochEmergencyFallbackTriggered(&triggered))
			assert.NoError(t, err)
			assert.True(t, triggered)

			// read the value of the block ID, should match
			var storedBlockID flow.Identifier
			err = db.View(RetrieveEpochEmergencyFallbackTriggeredBlockID(&storedBlockID))
			assert.NoError(t, err)
			assert.Equal(t, blockID, storedBlockID)
		})
	})
	t.Run("setting flag multiple time should have no additional effect", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			// set the flag, ensure no error
			err := db.Update(SetEpochEmergencyFallbackTriggered(blockID))
			assert.NoError(t, err)

			// set the flag, should have no error and no effect on state
			err = db.Update(SetEpochEmergencyFallbackTriggered(unittest.IdentifierFixture()))
			assert.NoError(t, err)

			// read the flag, should be true
			var triggered bool
			err = db.View(CheckEpochEmergencyFallbackTriggered(&triggered))
			assert.NoError(t, err)
			assert.True(t, triggered)

			// read the value of block ID, should equal the FIRST set ID
			var storedBlockID flow.Identifier
			err = db.View(RetrieveEpochEmergencyFallbackTriggeredBlockID(&storedBlockID))
			assert.NoError(t, err)
			assert.Equal(t, blockID, storedBlockID)
		})
	})
}

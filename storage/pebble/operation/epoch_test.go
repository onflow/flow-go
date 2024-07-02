package operation

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEpochEmergencyFallback(t *testing.T) {

	// the block ID where EFM was triggered
	blockID := unittest.IdentifierFixture()

	t.Run("reading when unset should return false", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			var triggered bool
			err := CheckEpochEmergencyFallbackTriggered(&triggered)(db)
			assert.NoError(t, err)
			assert.False(t, triggered)
		})
	})
	t.Run("should be able to set flag to true", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			// set the flag, ensure no error
			err := SetEpochEmergencyFallbackTriggered(blockID)(db)
			assert.NoError(t, err)

			// read the flag, should be true now
			var triggered bool
			err = CheckEpochEmergencyFallbackTriggered(&triggered)(db)
			assert.NoError(t, err)
			assert.True(t, triggered)

			// read the value of the block ID, should match
			var storedBlockID flow.Identifier
			err = RetrieveEpochEmergencyFallbackTriggeredBlockID(&storedBlockID)(db)
			assert.NoError(t, err)
			assert.Equal(t, blockID, storedBlockID)
		})
	})
}

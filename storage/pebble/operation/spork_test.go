package operation

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSporkID_InsertRetrieve(t *testing.T) {
	unittest.RunWithWrappedPebbleDB(t, func(db *unittest.PebbleWrapper) {
		sporkID := unittest.IdentifierFixture()

		err := db.Update(InsertSporkID(sporkID))
		require.NoError(t, err)

		var actual flow.Identifier
		err = db.View(RetrieveSporkID(&actual))
		require.NoError(t, err)

		assert.Equal(t, sporkID, actual)
	})
}

func TestProtocolVersion_InsertRetrieve(t *testing.T) {
	unittest.RunWithWrappedPebbleDB(t, func(db *unittest.PebbleWrapper) {
		version := uint(rand.Uint32())

		err := db.Update(InsertProtocolVersion(version))
		require.NoError(t, err)

		var actual uint
		err = db.View(RetrieveProtocolVersion(&actual))
		require.NoError(t, err)

		assert.Equal(t, version, actual)
	})
}

// TestEpochCommitSafetyThreshold_InsertRetrieve tests that we can insert and
// retrieve epoch commit safety threshold values.
func TestEpochCommitSafetyThreshold_InsertRetrieve(t *testing.T) {
	unittest.RunWithWrappedPebbleDB(t, func(db *unittest.PebbleWrapper) {
		threshold := rand.Uint64()

		err := db.Update(InsertEpochCommitSafetyThreshold(threshold))
		require.NoError(t, err)

		var actual uint64
		err = db.View(RetrieveEpochCommitSafetyThreshold(&actual))
		require.NoError(t, err)

		assert.Equal(t, threshold, actual)
	})
}

package operation

import (
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestHeaderInsertCheckRetrieve(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		expected := &flow.Header{
			View:               1337,
			Timestamp:          time.Now().UTC(),
			ParentID:           flow.Identifier{0x11},
			PayloadHash:        flow.Identifier{0x22},
			ParentVoterIndices: []byte{0x44},
			ParentVoterSigData: []byte{0x88},
			ProposerID:         flow.Identifier{0x33},
			ProposerSigData:    crypto.Signature{0x77},
		}
		blockID := expected.ID()

		err := InsertHeader(expected.ID(), expected)(db)
		require.Nil(t, err)

		var actual flow.Header
		err = RetrieveHeader(blockID, &actual)(db)
		require.Nil(t, err)

		assert.Equal(t, *expected, actual)
	})
}

func TestHeaderIDIndexByCollectionID(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

		headerID := unittest.IdentifierFixture()
		collectionID := unittest.IdentifierFixture()

		err := IndexCollectionBlock(collectionID, headerID)(db)
		require.Nil(t, err)

		actualID := &flow.Identifier{}
		err = LookupCollectionBlock(collectionID, actualID)(db)
		require.Nil(t, err)
		assert.Equal(t, headerID, *actualID)
	})
}

func TestBlockHeightIndexLookup(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

		height := uint64(1337)
		expected := flow.Identifier{0x01, 0x02, 0x03}

		err := IndexBlockHeight(height, expected)(db)
		require.Nil(t, err)

		var actual flow.Identifier
		err = LookupBlockHeight(height, &actual)(db)
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}

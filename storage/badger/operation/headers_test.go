// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestHeaderInsertCheckRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := flow.Header{
			View:               1337,
			Timestamp:          time.Now().UTC(),
			ParentID:           flow.Identifier{0x11},
			PayloadHash:        flow.Identifier{0x22},
			ParentVoterIDs:     []flow.Identifier{{0x44}},
			ParentVoterSigData: []byte{0x88},
			ProposerID:         flow.Identifier{0x33},
			ProposerSigData:    crypto.Signature{0x77},
		}
		blockID := expected.ID()

		err := db.Update(InsertHeader(expected.ID(), &expected))
		require.Nil(t, err)

		var actual flow.Header
		err = db.View(RetrieveHeader(blockID, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}

func TestHeaderIDIndexByCollectionID(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		headerID := unittest.IdentifierFixture()
		collectionID := unittest.IdentifierFixture()

		err := db.Update(IndexCollectionBlock(collectionID, headerID))
		require.Nil(t, err)

		actualID := &flow.Identifier{}
		err = db.View(LookupCollectionBlock(collectionID, actualID))
		require.Nil(t, err)
		assert.Equal(t, headerID, *actualID)
	})
}

func TestBlockHeightIndexLookup(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		height := uint64(1337)
		expected := flow.Identifier{0x01, 0x02, 0x03}

		err := db.Update(IndexBlockHeight(height, expected))
		require.Nil(t, err)

		var actual flow.Identifier
		err = db.View(LookupBlockHeight(height, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}

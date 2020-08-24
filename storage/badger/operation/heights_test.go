// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestFinalizedInsertUpdateRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		height := uint64(1337)

		err := db.Update(InsertFinalizedHeight(height))
		require.Nil(t, err)

		var retrieved uint64
		err = db.View(RetrieveFinalizedHeight(&retrieved))
		require.Nil(t, err)

		assert.Equal(t, retrieved, height)

		height = 9999
		err = db.Update(UpdateFinalizedHeight(height))
		require.Nil(t, err)

		err = db.View(RetrieveFinalizedHeight(&retrieved))
		require.Nil(t, err)

		assert.Equal(t, retrieved, height)
	})
}

func TestSealedInsertUpdateRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		height := uint64(1337)

		err := db.Update(InsertSealedHeight(height))
		require.Nil(t, err)

		var retrieved uint64
		err = db.View(RetrieveSealedHeight(&retrieved))
		require.Nil(t, err)

		assert.Equal(t, retrieved, height)

		height = 9999
		err = db.Update(UpdateSealedHeight(height))
		require.Nil(t, err)

		err = db.View(RetrieveSealedHeight(&retrieved))
		require.Nil(t, err)

		assert.Equal(t, retrieved, height)
	})
}

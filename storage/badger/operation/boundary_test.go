// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestBoundaryInsertUpdateRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(t *testing.T, db *badger.DB) {
		boundary := uint64(1337)

		err := db.Update(InsertBoundary(boundary))
		require.Nil(t, err)

		var retrieved uint64
		err = db.View(RetrieveBoundary(&retrieved))
		require.Nil(t, err)

		assert.Equal(t, retrieved, boundary)

		boundary = 9999
		err = db.Update(UpdateBoundary(boundary))
		require.Nil(t, err)

		err = db.View(RetrieveBoundary(&retrieved))
		require.Nil(t, err)

		assert.Equal(t, retrieved, boundary)
	})
}

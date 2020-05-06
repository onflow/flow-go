// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestIdentitiesInsertRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := unittest.IdentityListFixture(4)

		err := db.Update(InsertIdentities(expected))
		require.NoError(t, err)

		err = db.Update(InsertIdentities(expected))
		require.Error(t, err)
		assert.True(t, errors.Is(err, storage.ErrAlreadyExists))

		var actual flow.IdentityList
		err = db.View(RetrieveIdentities(&actual))
		require.NoError(t, err)

		assert.Equal(t, expected, actual)
	})
}

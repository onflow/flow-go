// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestIdentitiesInsertRetriev(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := unittest.IdentityListFixture(8)

		err := db.Update(InsertIdentities(expected))
		require.Nil(t, err)

		var actual flow.IdentityList
		err = db.View(RetrieveIdentities(&actual))
		require.Nil(t, err)

		assert.ElementsMatch(t, expected, actual)
	})
}

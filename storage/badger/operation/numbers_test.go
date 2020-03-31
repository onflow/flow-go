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

func TestNumberInsertRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(t *testing.T, db *badger.DB) {

		number := uint64(1337)
		expected := flow.Identifier{0x01, 0x02, 0x03}

		err := db.Update(InsertNumber(number, expected))
		require.Nil(t, err)

		var actual flow.Identifier
		err = db.View(RetrieveNumber(number, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}

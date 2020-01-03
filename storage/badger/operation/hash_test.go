// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestHashInsertRetrieve(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		number := uint64(1337)
		expected := crypto.Hash{0x01, 0x02, 0x03}

		err := db.Update(InsertHash(number, expected))
		require.Nil(t, err)

		var actual crypto.Hash
		err = db.View(RetrieveHash(number, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})

}

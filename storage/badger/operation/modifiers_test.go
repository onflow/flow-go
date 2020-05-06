// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestSkipDuplicates(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		e := Entity{ID: 1337}
		key := []byte{0x01, 0x02, 0x03}
		val, _ := msgpack.Marshal(e)

		// persist first time
		err := db.Update(insert(key, e))
		require.NoError(t, err)

		e2 := Entity{ID: 1338}

		// persist again
		err = db.Update(SkipDuplicates(insert(key, e2)))
		require.NoError(t, err)

		// ensure old value is still used
		var act []byte
		_ = db.View(func(tx *badger.Txn) error {
			item, err := tx.Get(key)
			require.NoError(t, err)
			act, err = item.ValueCopy(nil)
			require.NoError(t, err)
			return nil
		})

		assert.Equal(t, val, act)
	})
}

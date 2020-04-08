// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestStateInteractionsInsertCheckRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		d1 := delta.NewView(func(key flow.RegisterID) (value flow.RegisterValue, err error) {
			return nil, nil
		})

		d2 := delta.NewView(func(key flow.RegisterID) (value flow.RegisterValue, err error) {
			return nil, nil
		})

		// some set and reads
		d1.Set([]byte("\x89krg\u007fBN\x1d\xf5\xfb\xb8r\xbc4\xbd\x98ռ\xf1\xd0twU\xbf\x16N\xb4?,\xa0&;"), []byte("zażółć gęślą jaźń"))
		d1.Set([]byte{2}, []byte("b"))
		d1.Set([]byte{2}, []byte("c"))

		_, err := d1.Get([]byte{2})
		require.NoError(t, err)
		_, err = d1.Get([]byte{3})
		require.NoError(t, err)

		interactions := []*delta.Interactions{d1.Interactions(), d2.Interactions()}

		blockID := unittest.IdentifierFixture()

		err = db.Update(InsertExecutionStateInteractions(blockID, interactions))
		require.Nil(t, err)

		var readInteractions []*delta.Interactions

		err = db.View(RetrieveExecutionStateInteractions(blockID, &readInteractions))
		require.NoError(t, err)

		assert.Equal(t, interactions, readInteractions)

		assert.Equal(t, d1.Delta(), d1.Interactions().Delta)
		assert.Equal(t, d1.Reads(), d1.Interactions().Reads)
	})
}

package operation

import (
	"testing"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChilds(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		parent, child, notExist := flow.Identifier{0x11}, flow.Identifier{0x22}, flow.Identifier{0x33}

		// retrieve non exist should return not found
		var neverRetrieved flow.Identifier
		err := db.View(LookupBlockIDByParentID(notExist, &neverRetrieved))
		require.Error(t, err, badger.ErrKeyNotFound)

		err = db.Update(IndexBlockByParentID(parent, child))
		require.Nil(t, err)

		// should be able to retrieve back after indexed
		var retrieved flow.Identifier
		err = db.View(LookupBlockIDByParentID(parent, &retrieved))
		require.Nil(t, err)

		assert.Equal(t, child, retrieved)
	})
}

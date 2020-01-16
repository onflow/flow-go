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

func TestAddressInsertRetrieve(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		nodeID := flow.Identifier{0x01}
		expected := "address"

		err := db.Update(InsertAddress(nodeID, expected))
		require.Nil(t, err)

		var actual string
		err = db.View(RetrieveAddress(nodeID, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}

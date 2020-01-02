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

func TestRoleInsertRetrieve(t *testing.T) {

	unittest.RunWithDB(t, func(db *badger.DB) {
		nodeID := flow.Identifier{0x01}
		expected := flow.Role(13)

	err = db.Update(InsertRole(nodeID, expected))
	require.Nil(t, err)

		var actual flow.Role
		err = db.View(RetrieveRole(nodeID, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}

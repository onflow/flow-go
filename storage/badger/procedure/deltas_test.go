// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package procedure

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestApplyDeltas(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(t *testing.T, db *badger.DB) {
		block := unittest.BlockFixture()

		err := db.Update(ApplyDeltas(block.View, block.Identities))
		require.Nil(t, err)

		for _, id := range block.Identities {
			var delta int64
			err := db.View(operation.RetrieveDelta(block.View, id.Role, id.ID(), &delta))
			require.NoError(t, err)
			require.Equal(t, int64(id.Stake), delta)
		}
	})
}

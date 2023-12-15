// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestStateInteractionsInsertCheckRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		id1 := flow.NewRegisterID(
			flow.BytesToAddress([]byte("\x89krg\u007fBN\x1d\xf5\xfb\xb8r\xbc4\xbd\x98ռ\xf1\xd0twU\xbf\x16N\xb4?,\xa0&;")),
			"")
		id2 := flow.NewRegisterID(flow.BytesToAddress([]byte{2}), "")
		id3 := flow.NewRegisterID(flow.BytesToAddress([]byte{3}), "")

		executionSnapshot := &snapshot.ExecutionSnapshot{
			ReadSet: map[flow.RegisterID]struct{}{
				id2: {},
				id3: {},
			},
			WriteSet: map[flow.RegisterID]flow.RegisterValue{
				id1: []byte("zażółć gęślą jaźń"),
				id2: []byte("c"),
			},
		}

		interactions := []*snapshot.ExecutionSnapshot{
			executionSnapshot,
			{},
		}

		blockID := unittest.IdentifierFixture()

		err := db.Update(InsertExecutionStateInteractions(blockID, interactions))
		require.Nil(t, err)

		var readInteractions []*snapshot.ExecutionSnapshot

		err = db.View(RetrieveExecutionStateInteractions(blockID, &readInteractions))
		require.NoError(t, err)

		assert.Equal(t, interactions, readInteractions)
		assert.Equal(
			t,
			executionSnapshot.WriteSet,
			readInteractions[0].WriteSet)
		assert.Equal(
			t,
			executionSnapshot.ReadSet,
			readInteractions[0].ReadSet)
	})
}

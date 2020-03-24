package procedure

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestInsertIndexRetreivePayload(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		block := unittest.BlockFixture()

		err := db.Update(operation.InsertHeader(&block.Header))
		require.NoError(t, err)

		err = db.Update(InsertPayload(&block.Payload))
		require.NoError(t, err)

		err = db.Update(IndexPayload(&block.Header, &block.Payload))
		require.NoError(t, err)

		var retrieved flow.Payload
		err = db.View(RetrievePayload(block.ID(), &retrieved))
		require.NoError(t, err)

		require.Equal(t, block.Payload, retrieved)
	})
}

package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestTransactionInfo(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := unittest.TransactionInfoFixture()
		err := db.Update(InsertTransactionInfo(&expected))
		require.Nil(t, err)

		var actual flow.TransactionInfo
		err = db.View(RetrieveTransactionInfo(expected.ID(), &actual))
		require.Nil(t, err)
		assert.Equal(t, expected, actual)
	})
}

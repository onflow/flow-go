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

func TestReceipts_InsertRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		receipt := unittest.ExecutionReceiptFixture()
		expected := receipt.Meta()

		err := db.Update(InsertExecutionReceiptMeta(receipt.ID(), expected))
		require.Nil(t, err)

		var actual flow.ExecutionReceiptMeta
		err = db.View(RetrieveExecutionReceiptMeta(receipt.ID(), &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, &actual)
	})
}

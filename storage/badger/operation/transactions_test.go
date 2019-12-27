package operation

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestTransactions(t *testing.T) {

	unittest.RunWithDB(t, func(db *badger.DB) {
		expected := unittest.TransactionFixture()
		err = db.Update(operation.InsertTransaction(expected.Fingerprint(), &expected))
		require.Nil(t, err)

		var actual flow.Transaction
		err = db.View(operation.RetrieveTransaction(expected.Fingerprint(), &actual))
		require.Nil(t, err)
		assert.Equal(t, expected, actual)

		err = db.Update(operation.RemoveTransaction(expected.Hash()))
		require.Nil(t, err)

		err = db.View(operation.RetrieveTransaction(expected.Fingerprint(), &actual))
		// should fail since this was just deleted
		if assert.Error(t, err) {
			assert.True(t, errors.Is(err, storage.NotFoundErr))
		}
	})
}

package operation_test

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
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestTransactions(t *testing.T) {

	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))
	defer os.RemoveAll(dir)
	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.Nil(t, err)

	expected := unittest.TransactionFixture()
	err = db.Update(operation.InsertTransaction(expected.Hash(), &expected))
	require.Nil(t, err)

	var actual flow.Transaction
	err = db.View(operation.RetrieveTransaction(expected.Hash(), &actual))
	require.Nil(t, err)
	assert.Equal(t, expected, actual)

	err = db.Update(operation.RemoveTransaction(expected.Hash()))
	require.Nil(t, err)

	err = db.View(operation.RetrieveTransaction(expected.Hash(), &actual))
	// should fail since this was just deleted
	if assert.Error(t, err) {
		assert.True(t, errors.Is(err, badger.ErrKeyNotFound))
	}
}

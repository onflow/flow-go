package badger_test

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

	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestTransactions(t *testing.T) {
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))
	defer os.RemoveAll(dir)
	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.Nil(t, err)

	store := storage.NewTransactions(db)

	expected := unittest.TransactionFixture()
	err = store.Insert(&expected)
	require.Nil(t, err)

	actual, err := store.ByHash(expected.Hash())
	require.Nil(t, err)

	assert.Equal(t, &expected, actual)

	err = store.Remove(expected.Hash())
	require.NoError(t, err)

	// should fail since this was just deleted
	_, err = store.ByHash(expected.Hash())
	if assert.Error(t, err) {
		assert.True(t, errors.Is(err, badger.ErrKeyNotFound))
	}
}

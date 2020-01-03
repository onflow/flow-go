package badger_test

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestGetCollectionTransactions(t *testing.T) {
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))
	defer os.RemoveAll(dir)
	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.Nil(t, err)

	txStore := storage.NewTransactions(db)
	collStore := storage.NewCollections(db)

	tx1 := unittest.TransactionFixture(func(t *flow.Transaction) {
		t.Nonce = 1
	})
	tx2 := unittest.TransactionFixture(func(t *flow.Transaction) {
		t.Nonce = 2
	})
	collection := flow.Collection{
		Transactions: []flow.Fingerprint{tx1.Fingerprint(), tx2.Fingerprint()},
	}

	err = txStore.Insert(&tx1)
	assert.NoError(t, err)
	err = txStore.Insert(&tx2)
	assert.NoError(t, err)
	collStore.Insert(&collection)
	assert.NoError(t, err)

	actual, err := collStore.ByFingerprint(collection.Fingerprint())
	assert.NoError(t, err)
	assert.Equal(t, &collection, actual)

	transactions, err := collStore.TransactionsByFingerprint(collection.Fingerprint())
	assert.NoError(t, err)
	assert.Len(t, transactions, 2)
	assert.Equal(t, tx1, *transactions[0])
	assert.Equal(t, tx2, *transactions[1])
}

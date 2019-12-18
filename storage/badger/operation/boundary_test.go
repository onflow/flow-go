// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage"
)

func wrap(fn func(txn *badger.Txn) storage.Error) func(txn *badger.Txn) error {
	return func(txn *badger.Txn) error {
		return fn(txn)
	}
}

func TestBoundaryInsertUpdateRetrieve(t *testing.T) {

	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))
	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.Nil(t, err)

	boundary := uint64(1337)

	err = db.Update(wrap(InsertNewBoundary(boundary)))
	require.Nil(t, err)

	var retrieved uint64
	err = db.View(wrap(RetrieveBoundary(&retrieved)))
	require.Nil(t, err)

	assert.Equal(t, retrieved, boundary)

	boundary = 9999
	err = db.Update(wrap(UpdateBoundary(boundary)))
	require.Nil(t, err)

	err = db.View(wrap(RetrieveBoundary(&retrieved)))
	require.Nil(t, err)

	assert.Equal(t, retrieved, boundary)
}

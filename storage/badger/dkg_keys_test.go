package badger_test

import (
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestDKGKeysInsertAndRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := badgerstorage.NewDKGKeys(metrics, db)

		rand.Seed(time.Now().UnixNano())
		epochCounter := rand.Uint64()

		// attempt to get a non-existent key
		_, err := store.RetrieveMyDKGPrivateInfo(epochCounter)
		assert.True(t, errors.Is(err, storage.ErrNotFound))

		// store a key in db
		expected := unittest.DKGParticipantPriv()
		err = store.InsertMyDKGPrivateInfo(epochCounter, expected)
		require.NoError(t, err)

		// retrieve the key by epoch counter
		actual, err := store.RetrieveMyDKGPrivateInfo(epochCounter)
		require.NoError(t, err)
		assert.Equal(t, expected, actual)

		// test storing same key
		err = store.InsertMyDKGPrivateInfo(epochCounter, expected)
		require.True(t, errors.Is(err, storage.ErrAlreadyExists))
	})
}

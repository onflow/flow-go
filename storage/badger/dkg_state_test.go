package badger_test

import (
	"math/rand"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestDKGState(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		store, err := bstorage.NewDKGState(db)
		require.NoError(t, err)

		epochCounter := rand.Uint64()

		// check dkg-started flag for non-existent epoch
		started, err := store.GetDKGStarted(rand.Uint64())
		assert.NoError(t, err)
		assert.False(t, started)

		// store dkg-started flag for epoch
		err = store.SetDKGStarted(epochCounter)
		assert.NoError(t, err)

		// retrieve flag for epoch
		started, err = store.GetDKGStarted(epochCounter)
		assert.NoError(t, err)
		assert.True(t, started)
	})
}

package badger

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/transaction"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestProtocolStateStoreRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()

		setups := NewEpochSetups(metrics, db)
		commits := NewEpochCommits(metrics, db)
		store := NewProtocolState(metrics, setups, commits, db, DefaultCacheSize)

		expected := unittest.ProtocolStateFixture()
		protocolStateID := expected.ID()

		// store protocol state and auxiliary info
		err := transaction.Update(db, func(tx *transaction.Tx) error {

			err := setups.StoreTx(expected.PreviousEpochSetup)(tx)
			require.NoError(t, err)
			err = setups.StoreTx(expected.CurrentEpochSetup)(tx)
			require.NoError(t, err)
			err = commits.StoreTx(expected.PreviousEpochCommit)(tx)
			require.NoError(t, err)
			err = commits.StoreTx(expected.CurrentEpochCommit)(tx)
			require.NoError(t, err)

			return store.StoreTx(protocolStateID, &expected.ProtocolStateEntry)(tx)
		})
		require.NoError(t, err)

		// fetch protocol state
		actual, err := store.ByID(protocolStateID)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}

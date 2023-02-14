package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestQuorumCertificates_StoreTx(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewQuorumCertificates(metrics, db, 10)
		qc := unittest.QuorumCertificateFixture()

		err := operation.RetryOnConflictTx(db, transaction.Update, store.StoreTx(qc))
		require.NoError(t, err)

		actual, err := store.ByBlockID(qc.BlockID)
		require.NoError(t, err)

		require.Equal(t, qc, actual)
	})
}

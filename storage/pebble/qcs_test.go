package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestQuorumCertificates_StoreTx tests storing and retrieving of QC.
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

// TestQuorumCertificates_StoreTx_OtherQC checks if storing other QC for same blockID results in
// expected storage error and already stored value is not overwritten.
func TestQuorumCertificates_StoreTx_OtherQC(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewQuorumCertificates(metrics, db, 10)
		qc := unittest.QuorumCertificateFixture()
		otherQC := unittest.QuorumCertificateFixture(func(otherQC *flow.QuorumCertificate) {
			otherQC.View = qc.View
			otherQC.BlockID = qc.BlockID
		})

		err := operation.RetryOnConflictTx(db, transaction.Update, store.StoreTx(qc))
		require.NoError(t, err)

		err = operation.RetryOnConflictTx(db, transaction.Update, store.StoreTx(otherQC))
		require.ErrorIs(t, err, storage.ErrAlreadyExists)

		actual, err := store.ByBlockID(otherQC.BlockID)
		require.NoError(t, err)

		require.Equal(t, qc, actual)
	})
}

// TestQuorumCertificates_ByBlockID that ByBlockID returns correct sentinel error if no QC for given block ID has been found
func TestQuorumCertificates_ByBlockID(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		store := bstorage.NewQuorumCertificates(metrics, db, 10)

		actual, err := store.ByBlockID(unittest.IdentifierFixture())
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.Nil(t, actual)
	})
}

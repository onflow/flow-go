package store_test

import (
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestQuorumCertificates_StoreTx tests storing and retrieving of QC.
func TestQuorumCertificates_StoreTx(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		store := store.NewQuorumCertificates(metrics, db, 10)
		qc := unittest.QuorumCertificateFixture()

		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store.BatchStore(lctx, rw, qc)
			})
		})
		require.NoError(t, err)

		actual, err := store.ByBlockID(qc.BlockID)
		require.NoError(t, err)

		require.Equal(t, qc, actual)
	})
}

// TestQuorumCertificates_LockEnforced verifies that storing a QC requires holding the
// storage.LockInsertBlock lock. If the lock is not held, `BatchStore` should error.
func TestQuorumCertificates_LockEnforced(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		store := store.NewQuorumCertificates(metrics, db, 10)
		qc := unittest.QuorumCertificateFixture()

		// acquire wrong lock and attempt to store QC: should error
		err := unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error { // INCORRECT LOCK
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return store.BatchStore(lctx, rw, qc)
			})
		})
		// handle the error outside of the db.WithReaderBatchWriter, otherwise batch writer would
		// consider no error happened and would commit the batch, which would execute
		// the callbacks to store the QC in the cache, and causing the following reads
		// with ByBlockID to read from dirty data.
		require.Error(t, err)

		// qc should not be stored, so ByBlockID should return `storage.ErrNotFound`
		_, err = store.ByBlockID(qc.BlockID)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})
}

// TestQuorumCertificates_StoreTx_OtherQC checks if storing other QC for same blockID results in
// `storage.ErrAlreadyExists` and already stored value is not overwritten.
func TestQuorumCertificates_StoreTx_OtherQC(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		s := store.NewQuorumCertificates(metrics, db, 10)
		qc := unittest.QuorumCertificateFixture()
		otherQC := unittest.QuorumCertificateFixture(func(otherQC *flow.QuorumCertificate) {
			otherQC.View = qc.View
			otherQC.BlockID = qc.BlockID
		})

		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return s.BatchStore(lctx, rw, qc)
			})
		})
		require.NoError(t, err)

		err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return s.BatchStore(lctx, rw, otherQC)
			})
		})
		require.ErrorIs(t, err, storage.ErrAlreadyExists)

		actual, err := s.ByBlockID(otherQC.BlockID)
		require.NoError(t, err)

		require.Equal(t, qc, actual)
	})
}

// TestQuorumCertificates_ByBlockID that ByBlockID returns correct sentinel error if no QC for given block ID has been found
func TestQuorumCertificates_ByBlockID(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		store := store.NewQuorumCertificates(metrics, db, 10)

		actual, err := store.ByBlockID(unittest.IdentifierFixture())
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.Nil(t, actual)
	})
}

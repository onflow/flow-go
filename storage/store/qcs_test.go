package store_test

import (
	"testing"

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
		metrics := metrics.NewNoopCollector()
		store := store.NewQuorumCertificates(metrics, db, 10)
		qc := unittest.QuorumCertificateFixture()

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return store.BatchStore(rw, qc)
		})
		require.NoError(t, err)

		actual, err := store.ByBlockID(qc.BlockID)
		require.NoError(t, err)

		require.Equal(t, qc, actual)
	})
}

// TestQuorumCertificates_StoreTx_OtherQC checks if storing other QC for same blockID results in
// expected storage error and already stored value is not overwritten.
func TestQuorumCertificates_StoreTx_OtherQC(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		metrics := metrics.NewNoopCollector()
		s := store.NewQuorumCertificates(metrics, db, 10)
		qc := unittest.QuorumCertificateFixture()
		otherQC := unittest.QuorumCertificateFixture(func(otherQC *flow.QuorumCertificate) {
			otherQC.View = qc.View
			otherQC.BlockID = qc.BlockID
		})

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return s.BatchStore(rw, qc)
		})
		require.NoError(t, err)

		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return s.BatchStore(rw, otherQC)
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

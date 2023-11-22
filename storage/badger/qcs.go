package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// QuorumCertificates implements persistent storage for quorum certificates.
type QuorumCertificates struct {
	db    *badger.DB
	cache *Cache[flow.Identifier, *flow.QuorumCertificate]
}

var _ storage.QuorumCertificates = (*QuorumCertificates)(nil)

// NewQuorumCertificates Creates QuorumCertificates instance which is a database of quorum certificates
// which supports storing, caching and retrieving by block ID.
func NewQuorumCertificates(collector module.CacheMetrics, db *badger.DB, cacheSize uint) *QuorumCertificates {
	store := func(_ flow.Identifier, qc *flow.QuorumCertificate) func(*transaction.Tx) error {
		return transaction.WithTx(operation.InsertQuorumCertificate(qc))
	}

	retrieve := func(blockID flow.Identifier) func(tx *badger.Txn) (*flow.QuorumCertificate, error) {
		return func(tx *badger.Txn) (*flow.QuorumCertificate, error) {
			var qc flow.QuorumCertificate
			err := operation.RetrieveQuorumCertificate(blockID, &qc)(tx)
			return &qc, err
		}
	}

	return &QuorumCertificates{
		db: db,
		cache: newCache[flow.Identifier, *flow.QuorumCertificate](collector, metrics.ResourceQC,
			withLimit[flow.Identifier, *flow.QuorumCertificate](cacheSize),
			withStore(store),
			withRetrieve(retrieve)),
	}
}

func (q *QuorumCertificates) StoreTx(qc *flow.QuorumCertificate) func(*transaction.Tx) error {
	return q.cache.PutTx(qc.BlockID, qc)
}

func (q *QuorumCertificates) ByBlockID(blockID flow.Identifier) (*flow.QuorumCertificate, error) {
	tx := q.db.NewTransaction(false)
	defer tx.Discard()
	return q.retrieveTx(blockID)(tx)
}

func (q *QuorumCertificates) retrieveTx(blockID flow.Identifier) func(*badger.Txn) (*flow.QuorumCertificate, error) {
	return func(tx *badger.Txn) (*flow.QuorumCertificate, error) {
		val, err := q.cache.Get(blockID)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

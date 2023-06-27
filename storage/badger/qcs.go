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
	cache *Cache
}

var _ storage.QuorumCertificates = (*QuorumCertificates)(nil)

// NewQuorumCertificates Creates QuorumCertificates instance which is a database of quorum certificates
// which supports storing, caching and retrieving by block ID.
func NewQuorumCertificates(collector module.CacheMetrics, db *badger.DB, cacheSize uint) *QuorumCertificates {
	store := func(key interface{}, val interface{}) func(*transaction.Tx) error {
		qc := val.(*flow.QuorumCertificate)
		return transaction.WithTx(operation.InsertQuorumCertificate(qc))
	}

	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		blockID := key.(flow.Identifier)
		var qc flow.QuorumCertificate
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveQuorumCertificate(blockID, &qc)(tx)
			return &qc, err
		}
	}

	return &QuorumCertificates{
		db: db,
		cache: newCache(collector, metrics.ResourceQC,
			withLimit(cacheSize),
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
		return val.(*flow.QuorumCertificate), nil
	}
}

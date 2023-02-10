package badger

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type QuorumCertificates struct {
	db    *badger.DB
	cache *Cache
}

var _ storage.QuorumCertificates = (*QuorumCertificates)(nil)

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

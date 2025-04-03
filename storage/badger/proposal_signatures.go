package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// ProposalSignatures implements a proposal signature storage around a badger DB.
type ProposalSignatures struct {
	db    *badger.DB
	cache *Cache[flow.Identifier, []byte]
}

var _ storage.ProposalSignatures = (*ProposalSignatures)(nil)

// NewProposalSignatures creates a ProposalSignatures instance which is a database of block proposal signatures
// which supports storing, caching and retrieving by block ID.
func NewProposalSignatures(collector module.CacheMetrics, db *badger.DB) *ProposalSignatures {
	store := func(blockID flow.Identifier, sig []byte) func(*transaction.Tx) error {
		return transaction.WithTx(operation.InsertProposalSignature(blockID, &sig))
	}

	retrieve := func(blockID flow.Identifier) func(tx *badger.Txn) ([]byte, error) {
		var sig []byte
		return func(tx *badger.Txn) ([]byte, error) {
			err := operation.RetrieveProposalSignature(blockID, &sig)(tx)
			return sig, err
		}
	}

	return &ProposalSignatures{
		db: db,
		cache: newCache(collector, metrics.ResourceProposalSignature,
			withLimit[flow.Identifier, []byte](4*flow.DefaultTransactionExpiry),
			withStore(store),
			withRetrieve(retrieve)),
	}
}

func (h *ProposalSignatures) storeTx(blockID flow.Identifier, sig []byte) func(*transaction.Tx) error {
	return h.cache.PutTx(blockID, sig)
}

func (h *ProposalSignatures) retrieveTx(blockID flow.Identifier) func(*badger.Txn) ([]byte, error) {
	return h.cache.Get(blockID)
}

func (h *ProposalSignatures) ByBlockID(blockID flow.Identifier) ([]byte, error) {
	tx := h.db.NewTransaction(false)
	defer tx.Discard()
	return h.retrieveTx(blockID)(tx)
}

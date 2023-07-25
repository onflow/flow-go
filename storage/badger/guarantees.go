package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// Guarantees implements persistent storage for collection guarantees.
type Guarantees struct {
	db    *badger.DB
	cache *Cache[flow.Identifier, *flow.CollectionGuarantee]
}

func NewGuarantees(collector module.CacheMetrics, db *badger.DB, cacheSize uint) *Guarantees {

	store := func(collID flow.Identifier, guarantee *flow.CollectionGuarantee) func(*transaction.Tx) error {
		return transaction.WithTx(operation.SkipDuplicates(operation.InsertGuarantee(collID, guarantee)))
	}

	retrieve := func(collID flow.Identifier) func(*badger.Txn) (*flow.CollectionGuarantee, error) {
		var guarantee flow.CollectionGuarantee
		return func(tx *badger.Txn) (*flow.CollectionGuarantee, error) {
			err := operation.RetrieveGuarantee(collID, &guarantee)(tx)
			return &guarantee, err
		}
	}

	g := &Guarantees{
		db: db,
		cache: newCache[flow.Identifier, *flow.CollectionGuarantee](collector, metrics.ResourceGuarantee,
			withLimit[flow.Identifier, *flow.CollectionGuarantee](cacheSize),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return g
}

func (g *Guarantees) storeTx(guarantee *flow.CollectionGuarantee) func(*transaction.Tx) error {
	return g.cache.PutTx(guarantee.ID(), guarantee)
}

func (g *Guarantees) retrieveTx(collID flow.Identifier) func(*badger.Txn) (*flow.CollectionGuarantee, error) {
	return func(tx *badger.Txn) (*flow.CollectionGuarantee, error) {
		val, err := g.cache.Get(collID)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

func (g *Guarantees) Store(guarantee *flow.CollectionGuarantee) error {
	return operation.RetryOnConflictTx(g.db, transaction.Update, g.storeTx(guarantee))
}

func (g *Guarantees) ByCollectionID(collID flow.Identifier) (*flow.CollectionGuarantee, error) {
	tx := g.db.NewTransaction(false)
	defer tx.Discard()
	return g.retrieveTx(collID)(tx)
}

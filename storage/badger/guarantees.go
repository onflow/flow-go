package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// Guarantees implements persistent storage for collection guarantees.
type Guarantees struct {
	db    *badger.DB
	cache *Cache
}

func NewGuarantees(collector module.CacheMetrics, db *badger.DB) *Guarantees {

	store := func(collID flow.Identifier, v interface{}) func(*badger.Txn) error {
		guarantee := v.(*flow.CollectionGuarantee)
		return operation.SkipDuplicates(operation.InsertGuarantee(collID, guarantee))
	}

	retrieve := func(collID flow.Identifier) func(*badger.Txn) (interface{}, error) {
		var guarantee flow.CollectionGuarantee
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveGuarantee(collID, &guarantee)(tx)
			return &guarantee, err
		}
	}

	g := &Guarantees{
		db: db,
		cache: newCache(collector,
			withLimit(flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceGuarantee)),
	}

	return g
}

func (g *Guarantees) storeTx(guarantee *flow.CollectionGuarantee) func(*badger.Txn) error {
	return g.cache.Put(guarantee.ID(), guarantee)
}

func (g *Guarantees) retrieveTx(collID flow.Identifier) func(*badger.Txn) (*flow.CollectionGuarantee, error) {
	return func(tx *badger.Txn) (*flow.CollectionGuarantee, error) {
		v, err := g.cache.Get(collID)(tx)
		return v.(*flow.CollectionGuarantee), err
	}
}

func (g *Guarantees) Store(guarantee *flow.CollectionGuarantee) error {
	return operation.RetryOnConflict(g.db.Update, g.storeTx(guarantee))
}

func (g *Guarantees) ByCollectionID(collID flow.Identifier) (*flow.CollectionGuarantee, error) {
	return g.retrieveTx(collID)(g.db.NewTransaction(false))
}

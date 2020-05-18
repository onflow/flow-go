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

	store := func(collID flow.Identifier, guarantee interface{}) error {
		return operation.RetryOnConflict(db.Update, operation.SkipDuplicates(operation.InsertGuarantee(collID, guarantee.(*flow.CollectionGuarantee))))
	}

	retrieve := func(collID flow.Identifier) (interface{}, error) {
		var guarantee flow.CollectionGuarantee
		err := db.View(operation.RetrieveGuarantee(collID, &guarantee))
		return &guarantee, err
	}

	g := &Guarantees{
		db: db,
		cache: newCache(collector,
			withLimit(10*(flow.DefaultTransactionExpiry+100)),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceGuarantee),
		),
	}

	return g
}

func (g *Guarantees) Store(guarantee *flow.CollectionGuarantee) error {
	return g.cache.Put(guarantee.ID(), guarantee)
}

func (g *Guarantees) ByCollectionID(collID flow.Identifier) (*flow.CollectionGuarantee, error) {
	guarantee, err := g.cache.Get(collID)
	if err != nil {
		return nil, err
	}
	return guarantee.(*flow.CollectionGuarantee), nil
}

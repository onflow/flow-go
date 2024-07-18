package pebble

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

// Guarantees implements persistent storage for collection guarantees.
type Guarantees struct {
	db    *pebble.DB
	cache *Cache[flow.Identifier, *flow.CollectionGuarantee]
}

func NewGuarantees(collector module.CacheMetrics, db *pebble.DB, cacheSize uint) *Guarantees {

	store := func(collID flow.Identifier, guarantee *flow.CollectionGuarantee) func(storage.PebbleReaderBatchWriter) error {
		return storage.OnlyWriter(operation.InsertGuarantee(collID, guarantee))
	}

	retrieve := func(collID flow.Identifier) func(pebble.Reader) (*flow.CollectionGuarantee, error) {
		var guarantee flow.CollectionGuarantee
		return func(tx pebble.Reader) (*flow.CollectionGuarantee, error) {
			err := operation.RetrieveGuarantee(collID, &guarantee)(tx)
			return &guarantee, err
		}
	}

	g := &Guarantees{
		db: db,
		cache: newCache(collector, metrics.ResourceGuarantee,
			withLimit[flow.Identifier, *flow.CollectionGuarantee](cacheSize),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return g
}

func (g *Guarantees) storeTx(guarantee *flow.CollectionGuarantee) func(storage.PebbleReaderBatchWriter) error {
	return g.cache.PutPebble(guarantee.ID(), guarantee)
}

func (g *Guarantees) retrieveTx(collID flow.Identifier) func(pebble.Reader) (*flow.CollectionGuarantee, error) {
	return func(tx pebble.Reader) (*flow.CollectionGuarantee, error) {
		val, err := g.cache.Get(collID)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

func (g *Guarantees) Store(guarantee *flow.CollectionGuarantee) error {
	return operation.WithReaderBatchWriter(g.db, g.storeTx(guarantee))
}

func (g *Guarantees) ByCollectionID(collID flow.Identifier) (*flow.CollectionGuarantee, error) {
	return g.retrieveTx(collID)(g.db)
}

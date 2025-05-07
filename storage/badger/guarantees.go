package store

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// Guarantees implements persistent storage for collection guarantees.
type Guarantees struct {
	db    storage.DB
	cache *Cache[flow.Identifier, *flow.CollectionGuarantee]
}

func NewGuarantees(collector module.CacheMetrics, db storage.DB, cacheSize uint) *Guarantees {

	store := func(rw storage.ReaderBatchWriter, collID flow.Identifier, guarantee *flow.CollectionGuarantee) error {
		// insert when not found
		return operation.UnsafeInsertGuarantee(rw.Writer(), collID, guarantee)
	}

	retrieve := func(r storage.Reader, collID flow.Identifier) (*flow.CollectionGuarantee, error) {
		var guarantee flow.CollectionGuarantee
		err := operation.RetrieveGuarantee(r, collID, &guarantee)
		return &guarantee, err
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

func (g *Guarantees) storeTx(rw storage.ReaderBatchWriter, guarantee *flow.CollectionGuarantee) error {
	return g.cache.PutTx(rw, guarantee.ID(), guarantee)
}

func (g *Guarantees) retrieveTx(collID flow.Identifier) (*flow.CollectionGuarantee, error) {
	val, err := g.cache.Get(g.db.Reader(), collID)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (g *Guarantees) Store(guarantee *flow.CollectionGuarantee) error {
	return g.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return g.storeTx(rw, guarantee)
	})
}

func (g *Guarantees) ByCollectionID(collID flow.Identifier) (*flow.CollectionGuarantee, error) {
	return g.retrieveTx(collID)
}

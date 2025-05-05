package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// Guarantees implements persistent storage for collection guarantees.
type Guarantees struct {
	db *badger.DB

	// cache is essentially an in-memory map from `CollectionGuarantee.ID()` -> `CollectionGuarantee`
	cache *Cache[flow.Identifier, *flow.CollectionGuarantee]

	// byCollectionIdCache is essentially an in-memory map from `CollectionGuarantee.CollectionID` -> `CollectionGuarantee.ID()`.
	//The full flow.CollectionGuarantee can be retrieved from the `cache` above.
	byCollectionIdCache *Cache[flow.Identifier, flow.Identifier]
}

var _ storage.Guarantees = (*Guarantees)(nil)

// NewGuarantees creates a Guarantees instance, which stores collection guarantees.
// It supports storing, caching and retrieving by guaranteeID or the additionally indexed collection ID.
func NewGuarantees(
	collector module.CacheMetrics,
	db *badger.DB,
	cacheSize uint,
	byCollectionIDCacheSize uint,
) *Guarantees {

	storeByGuaranteeID := func(guaranteeID flow.Identifier, guarantee *flow.CollectionGuarantee) func(*transaction.Tx) error {
		return transaction.WithTx(operation.SkipDuplicates(operation.InsertGuarantee(guaranteeID, guarantee)))
	}

	retrieveByGuaranteeID := func(guaranteeID flow.Identifier) func(*badger.Txn) (*flow.CollectionGuarantee, error) {
		var guarantee flow.CollectionGuarantee
		return func(tx *badger.Txn) (*flow.CollectionGuarantee, error) {
			err := operation.RetrieveGuarantee(guaranteeID, &guarantee)(tx)
			return &guarantee, err
		}
	}

	indexByCollectionID := func(collID flow.Identifier, guaranteeID flow.Identifier) func(*transaction.Tx) error {
		return func(tx *transaction.Tx) error {
			err := transaction.WithTx(operation.IndexGuarantee(collID, guaranteeID))(tx)
			if err != nil {
				return fmt.Errorf("could not index guarantee for collection (%x): %w", collID[:], err)
			}
			return nil
		}
	}

	lookupByCollectionID := func(collID flow.Identifier) func(tx *badger.Txn) (flow.Identifier, error) {
		return func(tx *badger.Txn) (flow.Identifier, error) {
			var guaranteeID flow.Identifier
			err := operation.LookupGuarantee(collID, &guaranteeID)(tx)
			if err != nil {
				return flow.ZeroID, fmt.Errorf("could not lookup guarantee ID for collection (%x): %w", collID[:], err)
			}
			return guaranteeID, nil
		}
	}

	g := &Guarantees{
		db: db,
		cache: newCache[flow.Identifier, *flow.CollectionGuarantee](collector, metrics.ResourceGuarantee,
			withLimit[flow.Identifier, *flow.CollectionGuarantee](cacheSize),
			withStore(storeByGuaranteeID),
			withRetrieve(retrieveByGuaranteeID)),
		byCollectionIdCache: newCache[flow.Identifier, flow.Identifier](collector, metrics.ResourceGuaranteeByCollectionID,
			withLimit[flow.Identifier, flow.Identifier](byCollectionIDCacheSize),
			withStore(indexByCollectionID),
			withRetrieve(lookupByCollectionID)),
	}

	return g
}

func (g *Guarantees) storeTx(guarantee *flow.CollectionGuarantee) func(*transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		err := g.cache.PutTx(guarantee.ID(), guarantee)(tx)
		if err != nil {
			return err
		}

		err = g.byCollectionIdCache.PutTx(guarantee.CollectionID, guarantee.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index guarantee %x under collection %x: %w",
				guarantee.ID(), guarantee.CollectionID[:], err)
		}

		return nil
	}
}

func (g *Guarantees) retrieveTx(guaranteeID flow.Identifier) func(*badger.Txn) (*flow.CollectionGuarantee, error) {
	return func(tx *badger.Txn) (*flow.CollectionGuarantee, error) {
		val, err := g.cache.Get(guaranteeID)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

func (g *Guarantees) Store(guarantee *flow.CollectionGuarantee) error {
	return operation.RetryOnConflictTx(g.db, transaction.Update, g.storeTx(guarantee))
}

func (g *Guarantees) ByID(guaranteeID flow.Identifier) (*flow.CollectionGuarantee, error) {
	tx := g.db.NewTransaction(false)
	defer tx.Discard()
	return g.retrieveTx(guaranteeID)(tx)
}

func (g *Guarantees) ByCollectionID(collID flow.Identifier) (*flow.CollectionGuarantee, error) {
	tx := g.db.NewTransaction(false)
	defer tx.Discard()

	guaranteeID, err := g.byCollectionIdCache.Get(collID)(tx)
	if err != nil {
		return nil, fmt.Errorf("could not lookup collection guarantee ID for collection (%x): %w", collID[:], err)
	}

	return g.retrieveTx(guaranteeID)(tx)
}

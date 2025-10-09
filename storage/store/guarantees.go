package store

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// Guarantees implements persistent storage for collection guarantees.
type Guarantees struct {
	db storage.DB
	// cache is essentially an in-memory map from `CollectionGuarantee.ID()` -> `CollectionGuarantee`
	cache *Cache[flow.Identifier, *flow.CollectionGuarantee]

	// byCollectionIdCache is essentially an in-memory map from `CollectionGuarantee.CollectionID` -> `CollectionGuarantee.ID()`.
	// The full flow.CollectionGuarantee can be retrieved from the `cache` above.
	byCollectionIdCache *Cache[flow.Identifier, flow.Identifier]
}

var _ storage.Guarantees = (*Guarantees)(nil)

// NewGuarantees creates a Guarantees instance, which stores collection guarantees.
// It supports storing, caching and retrieving by guaranteeID or the additionally indexed collection ID.
func NewGuarantees(
	collector module.CacheMetrics,
	db storage.DB,
	cacheSize uint,
	byCollectionIDCacheSize uint,
) *Guarantees {

	storeByGuaranteeIDWithLock := func(rw storage.ReaderBatchWriter, guaranteeID flow.Identifier, guarantee *flow.CollectionGuarantee) error {
		return operation.InsertGuarantee(rw.Writer(), guaranteeID, guarantee)
	}

	retrieveByGuaranteeID := func(r storage.Reader, guaranteeID flow.Identifier) (*flow.CollectionGuarantee, error) {
		var guarantee flow.CollectionGuarantee
		err := operation.RetrieveGuarantee(r, guaranteeID, &guarantee)
		return &guarantee, err
	}

	// While a collection guarantee can only be present once in the finalized chain,
	// across different consensus forks we may encounter the same guarantee multiple times.
	// On the happy path there is a 1:1 correspondence between CollectionGuarantees and Collections.
	// However, the finalization status of guarantees is not yet verified by consensus nodes,
	// nor is the possibility of byzantine collection nodes dealt with, so we check here that
	// there are no conflicting guarantees for the same collection.

	lookupByCollectionID := func(r storage.Reader, collID flow.Identifier) (flow.Identifier, error) {
		var guaranteeID flow.Identifier
		err := operation.LookupGuarantee(r, collID, &guaranteeID)
		if err != nil {
			return flow.ZeroID, fmt.Errorf("could not lookup guarantee ID for collection (%x): %w", collID[:], err)
		}
		return guaranteeID, nil
	}

	g := &Guarantees{
		db: db,
		cache: newCache(collector, metrics.ResourceGuarantee,
			withLimit[flow.Identifier, *flow.CollectionGuarantee](cacheSize),
			withStore(storeByGuaranteeIDWithLock),
			withRetrieve(retrieveByGuaranteeID)),
		byCollectionIdCache: newCache[flow.Identifier, flow.Identifier](collector, metrics.ResourceGuaranteeByCollectionID,
			withLimit[flow.Identifier, flow.Identifier](byCollectionIDCacheSize),
			withStoreWithLock(operation.IndexGuarantee),
			withRetrieve(lookupByCollectionID)),
	}

	return g
}

func (g *Guarantees) storeTx(lctx lockctx.Proof, rw storage.ReaderBatchWriter, guarantee *flow.CollectionGuarantee) error {
	guaranteeID := guarantee.ID()
	err := g.cache.PutTx(rw, guaranteeID, guarantee)
	if err != nil {
		return err
	}

	err = g.byCollectionIdCache.PutWithLockTx(lctx, rw, guarantee.CollectionID, guaranteeID)
	if err != nil {
		return fmt.Errorf("could not index guarantee %x under collection %x: %w",
			guaranteeID, guarantee.CollectionID[:], err)
	}

	return nil
}

func (g *Guarantees) retrieveTx(guaranteeID flow.Identifier) (*flow.CollectionGuarantee, error) {
	val, err := g.cache.Get(g.db.Reader(), guaranteeID)
	if err != nil {
		return nil, err
	}
	return val, nil
}

// ByID returns the [flow.CollectionGuarantee] by its ID.
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no collection guarantee with the given Identifier is known.
func (g *Guarantees) ByID(guaranteeID flow.Identifier) (*flow.CollectionGuarantee, error) {
	return g.retrieveTx(guaranteeID)
}

// ByCollectionID retrieves the collection guarantee by collection ID.
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no collection guarantee has been indexed for the given collection ID.
func (g *Guarantees) ByCollectionID(collID flow.Identifier) (*flow.CollectionGuarantee, error) {
	guaranteeID, err := g.byCollectionIdCache.Get(g.db.Reader(), collID)
	if err != nil {
		return nil, fmt.Errorf("could not lookup collection guarantee ID for collection (%x): %w", collID[:], err)
	}

	return g.retrieveTx(guaranteeID)
}

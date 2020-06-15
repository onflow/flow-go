package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

const IdentitiesCacheSize = 500

type Identities struct {
	db    *badger.DB
	cache *Cache
}

func NewIdentities(collector module.CacheMetrics, db *badger.DB) *Identities {

	store := func(nodeID flow.Identifier, v interface{}) func(tx *badger.Txn) error {
		identity := v.(*flow.Identity)
		return operation.SkipDuplicates(operation.InsertIdentity(nodeID, identity))
	}

	retrieve := func(nodeID flow.Identifier) func(tx *badger.Txn) (interface{}, error) {
		var identity flow.Identity
		return func(tx *badger.Txn) (interface{}, error) {
			err := db.View(operation.RetrieveIdentity(nodeID, &identity))
			return &identity, err
		}
	}

	i := &Identities{
		db: db,
		cache: newCache(collector,
			withLimit(IdentitiesCacheSize),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceIdentity)),
	}

	return i
}

func (i *Identities) storeTx(identity *flow.Identity) func(*badger.Txn) error {
	return i.cache.Put(identity.ID(), identity)
}

func (i *Identities) retrieveTx(nodeID flow.Identifier) func(*badger.Txn) (*flow.Identity, error) {
	return func(tx *badger.Txn) (*flow.Identity, error) {
		v, err := i.cache.Get(nodeID)(tx)
		if err != nil {
			return nil, err
		}
		return v.(*flow.Identity), nil
	}
}

func (i *Identities) Store(identity *flow.Identity) error {
	return operation.RetryOnConflict(i.db.Update, i.storeTx(identity))
}

func (i *Identities) ByNodeID(nodeID flow.Identifier) (*flow.Identity, error) {
	return i.retrieveTx(nodeID)(i.db.NewTransaction(false))
}

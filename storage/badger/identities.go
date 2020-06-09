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

	store := func(nodeID flow.Identifier, identity interface{}) error {
		return operation.RetryOnConflict(db.Update, operation.SkipDuplicates(operation.InsertIdentity(nodeID, identity.(*flow.Identity))))
	}

	retrieve := func(nodeID flow.Identifier) (interface{}, error) {
		var identity flow.Identity
		err := db.View(operation.RetrieveIdentity(nodeID, &identity))
		return &identity, err
	}

	i := &Identities{
		db: db,
		cache: newCache(collector,
			withLimit(IdentitiesCacheSize),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceIdentity),
		),
	}
	return i
}

func (i *Identities) Store(identity *flow.Identity) error {
	return i.cache.Put(identity.NodeID, identity)
}

func (i *Identities) ByNodeID(nodeID flow.Identifier) (*flow.Identity, error) {
	identity, err := i.cache.Get(nodeID)
	if err != nil {
		return nil, err
	}
	return identity.(*flow.Identity), nil
}

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// Guarantees implements persistent storage for collection guarantees.
type Guarantees struct {
	db    *badger.DB
	cache *Cache
}

func NewGuarantees(db *badger.DB) *Guarantees {

	store := func(guarID flow.Identifier, guarantee interface{}) error {
		return db.Update(operation.InsertGuarantee(guarID, guarantee.(*flow.CollectionGuarantee)))
	}

	retrieve := func(guarID flow.Identifier) (interface{}, error) {
		var guarantee flow.CollectionGuarantee
		err := db.View(operation.RetrieveGuarantee(guarID, &guarantee))
		return &guarantee, err
	}

	g := &Guarantees{
		db:    db,
		cache: newCache(withLimit(10000), withStore(store), withRetrieve(retrieve)),
	}

	return g
}

func (g *Guarantees) Store(guarantee *flow.CollectionGuarantee) error {
	return g.cache.Put(guarantee.ID(), guarantee)
}

func (g *Guarantees) ByID(guarID flow.Identifier) (*flow.CollectionGuarantee, error) {
	guarantee, err := g.cache.Get(guarID)
	return guarantee.(*flow.CollectionGuarantee), err
}

func (g *Guarantees) ByBlockID(blockID flow.Identifier) ([]*flow.CollectionGuarantee, error) {
	var guarIDs []flow.Identifier
	err := g.db.View(operation.LookupPayloadGuarantees(blockID, &guarIDs))
	if err != nil {
		return nil, fmt.Errorf("could not lookup guarantees for block: %w", err)
	}
	guarantees := make([]*flow.CollectionGuarantee, 0, len(guarIDs))
	for _, guarID := range guarIDs {
		guarantee, err := g.ByID(guarID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve guarantee (%x): %w", guarID, err)
		}
		guarantees = append(guarantees, guarantee)
	}
	return guarantees, nil
}

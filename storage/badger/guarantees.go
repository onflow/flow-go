package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// Guarantees implements persistent storage for collection guarantees.
type Guarantees struct {
	db       *badger.DB
	payloads *Payloads
	cache    *Cache
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
		db:       db,
		payloads: NewPayloads(db),
		cache:    newCache(withLimit(10000), withStore(store), withRetrieve(retrieve)),
	}

	return g
}

func (g *Guarantees) Store(guarantee *flow.CollectionGuarantee) error {
	return g.cache.Put(guarantee.ID(), guarantee)
}

func (g *Guarantees) ByID(guarID flow.Identifier) (*flow.CollectionGuarantee, error) {
	guarantee, err := g.cache.Get(guarID)
	if err != nil {
		return nil, err
	}
	return guarantee.(*flow.CollectionGuarantee), nil
}

func (g *Guarantees) ByBlockID(blockID flow.Identifier) ([]*flow.CollectionGuarantee, error) {
	payload, err := g.payloads.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve block payload: %w", err)
	}
	return payload.Guarantees, nil
}

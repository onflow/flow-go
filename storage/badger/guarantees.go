package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// Guarantees implements persistent storage for collection guarantees.
type Guarantees struct {
	db *badger.DB
}

func NewGuarantees(db *badger.DB) *Guarantees {
	return &Guarantees{
		db: db,
	}
}

func (g *Guarantees) Store(guarantee *flow.CollectionGuarantee) error {
	return g.db.Update(func(tx *badger.Txn) error {
		err := operation.InsertGuarantee(guarantee)(tx)
		if err != nil {
			return fmt.Errorf("could not insert collection guarantee: %w", err)
		}
		return nil
	})
}

func (g *Guarantees) ByID(collID flow.Identifier) (*flow.CollectionGuarantee, error) {
	var guarantee flow.CollectionGuarantee

	err := g.db.View(func(tx *badger.Txn) error {
		return operation.RetrieveGuarantee(collID, &guarantee)(tx)
	})
	if err != nil {
		return nil, fmt.Errorf("could not retrieve collection guarantee: %w", err)
	}

	return &guarantee, nil
}

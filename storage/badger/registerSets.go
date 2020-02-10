package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type RegisterSets struct {
	db *badger.DB
}

func NewRegisterSets(db *badger.DB) *RegisterSets {
	return &RegisterSets{
		db: db,
	}
}

func (r *RegisterSets) Store(blockID flow.Identifier, set *flow.RegisterSet) error {
	return r.db.Update(func(btx *badger.Txn) error {
		err := operation.InsertRegisterSet(blockID, set)(btx)
		if err != nil {
			return fmt.Errorf("could not insert register set: %w", err)
		}
		return nil
	})
}

func (r *RegisterSets) ByBlockID(blockID flow.Identifier) (*flow.RegisterSet, error) {
	var set flow.RegisterSet

	err := r.db.View(func(btx *badger.Txn) error {
		err := operation.RetrieveRegisterSet(blockID, &set)(btx)
		if err != nil {
			if err == storage.ErrNotFound {
				return err
			}
			return fmt.Errorf("could not retrieve register set: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &set, nil
}

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type RegisterDeltas struct {
	db *badger.DB
}

func NewRegisterDeltas(db *badger.DB) *RegisterDeltas {
	return &RegisterDeltas{
		db: db,
	}
}

func (r *RegisterDeltas) Store(blockID flow.Identifier, delta *flow.RegisterDelta) error {
	return r.db.Update(func(btx *badger.Txn) error {
		err := operation.InsertRegisterDelta(blockID, delta)(btx)
		if err != nil {
			return fmt.Errorf("could not insert register delta: %w", err)
		}
		return nil
	})
}

func (r *RegisterDeltas) ByBlockID(blockID flow.Identifier) (*flow.RegisterDelta, error) {
	var delta flow.RegisterDelta

	err := r.db.View(func(btx *badger.Txn) error {
		err := operation.RetrieveRegisterDelta(blockID, &delta)(btx)
		if err != nil {
			if err == storage.ErrNotFound {
				return err
			}
			return fmt.Errorf("could not retrieve register delta: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &delta, nil
}

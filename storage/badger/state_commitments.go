package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type StateCommitments struct {
	db *badger.DB
}

func NewStateCommitments(db *badger.DB) *StateCommitments {
	return &StateCommitments{
		db: db,
	}
}

func (s *StateCommitments) Persist(id flow.Identifier, commitment *flow.StateCommitment) error {
	return s.db.Update(func(btx *badger.Txn) error {
		err := operation.PersistStateCommitment(id, commitment)(btx)
		if err != nil {
			return fmt.Errorf("could not insert state commitment: %w", err)
		}
		return nil
	})
}

func (s *StateCommitments) ByID(id flow.Identifier) (*flow.StateCommitment, error) {
	var commitment flow.StateCommitment
	err := s.db.View(func(btx *badger.Txn) error {
		err := operation.RetrieveStateCommitment(id, &commitment)(btx)
		if err != nil {
			if err == storage.ErrNotFound {
				return err
			}
			return fmt.Errorf("could not retrerieve state commitment: %w", err)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return &commitment, nil
}

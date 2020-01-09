package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
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

func (s *StateCommitments) Persist(hash crypto.Hash, commitment *flow.StateCommitment) error {
	return s.db.Update(func(btx *badger.Txn) error {
		err := operation.PersistStateCommitment(hash, commitment)(btx)
		if err != nil {
			return fmt.Errorf("could not insert state commitment: %w", err)
		}
		return nil
	})
}

func (s *StateCommitments) ByHash(hash crypto.Hash) (*flow.StateCommitment, error) {
	var commitment flow.StateCommitment
	err := s.db.View(func(btx *badger.Txn) error {
		err := operation.RetrieveStateCommitment(hash, &commitment)(btx)
		if err != nil {
			if err == storage.NotFoundErr {
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

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Commits struct {
	db *badger.DB
}

func NewCommits(db *badger.DB) *Commits {
	return &Commits{
		db: db,
	}
}

func (c *Commits) Store(blockID flow.Identifier, commit flow.StateCommitment) error {
	return c.db.Update(func(btx *badger.Txn) error {
		err := operation.IndexStateCommitment(blockID, commit)(btx)
		if err != nil {
			return fmt.Errorf("could not insert state commitment: %w", err)
		}
		return nil
	})
}

func (c *Commits) ByID(blockID flow.Identifier) (flow.StateCommitment, error) {
	var commitment flow.StateCommitment
	err := c.db.View(func(btx *badger.Txn) error {
		err := operation.LookupStateCommitment(blockID, &commitment)(btx)
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
	return commitment, nil
}

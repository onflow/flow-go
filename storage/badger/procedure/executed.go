package procedure

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// UpdateLastExecutedBlock updates the latest executed block to be the input block
func UpdateLastExecutedBlock(header *flow.Header) func(txn *badger.Txn) error {
	return func(txn *badger.Txn) error {
		err := operation.UpdateExecutedBlock(header.ID())(txn)
		if err != nil {
			return fmt.Errorf("cannot update highest executed block: %w", err)
		}

		return nil
	}
}

// GetLastExecutedBlock retrieves the height and ID of the latest block executed by this node.
// Returns storage.ErrNotFound if no latest executed block has been stored.
func GetLastExecutedBlock(height *uint64, blockID *flow.Identifier) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {
		var highest flow.Header
		err := operation.RetrieveExecutedBlock(blockID)(tx)
		if err != nil {
			return fmt.Errorf("could not lookup executed block %v: %w", blockID, err)
		}
		err = operation.RetrieveHeader(*blockID, &highest)(tx)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return fmt.Errorf("unexpected: latest executed block does not exist in storage: %s", err.Error())
			}
			return fmt.Errorf("could not retrieve executed header %v: %w", blockID, err)
		}
		*height = highest.Height
		return nil
	}
}

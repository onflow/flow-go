package procedure

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// UpdateHighestExecutedBlockIfHigher updates the latest executed block to be the input block
// if the input block has a greater height than the currently stored latest executed block.
// The executed block index must have been initialized before calling this function.
// Returns storage.ErrNotFound if the input block does not exist in storage.
func UpdateHighestExecutedBlockIfHigher(header *flow.Header) func(txn *badger.Txn) error {
	return func(txn *badger.Txn) error {
		var blockID flow.Identifier
		err := operation.RetrieveExecutedBlock(&blockID)(txn)
		if err != nil {
			return fmt.Errorf("cannot lookup executed block: %w", err)
		}

		var highest flow.Header
		err = operation.RetrieveHeader(blockID, &highest)(txn)
		if err != nil {
			return fmt.Errorf("cannot retrieve executed header: %w", err)
		}

		if header.Height <= highest.Height {
			return nil
		}
		err = operation.UpdateExecutedBlock(header.ID())(txn)
		if err != nil {
			return fmt.Errorf("cannot update highest executed block: %w", err)
		}

		return nil
	}
}

// GetHighestExecutedBlock retrieves the height and ID of the latest block executed by this node.
// Returns storage.ErrNotFound if no latest executed block has been stored.
func GetHighestExecutedBlock(height *uint64, blockID *flow.Identifier) func(tx *badger.Txn) error {
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

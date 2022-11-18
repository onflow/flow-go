package procedure

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/operation"
)

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

func GetHighestExecutedBlock(height *uint64, blockID *flow.Identifier) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {
		var highest flow.Header
		err := operation.RetrieveExecutedBlock(blockID)(tx)
		if err != nil {
			return fmt.Errorf("could not lookup executed block %v: %w", blockID, err)
		}
		err = operation.RetrieveHeader(*blockID, &highest)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve executed header %v: %w", blockID, err)
		}
		*height = highest.Height
		return nil
	}
}

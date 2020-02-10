// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package procedure

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

func InsertBlock(block *flow.Block) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// store the block header
		err := operation.InsertHeader(&block.Header)(tx)
		if err != nil {
			return fmt.Errorf("could not store block header: %w", err)
		}

		// index the block payload
		err = IndexPayload(&block.Payload)(tx)
		if err != nil {
			return fmt.Errorf("could not index block payload: %w", err)
		}

		return nil

	}
}

func RetrieveBlock(blockID flow.Identifier, block *flow.Block) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// get the block header
		err := operation.RetrieveHeader(blockID, &block.Header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header: %w", err)
		}

		// get the block payload
		err = RetrievePayload(block.Header.PayloadHash, &block.Payload)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve payload: %w", err)
		}

		return nil
	}
}

func FinalizeBlock(blockID flow.Identifier) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// retrieve the header to check the parent
		var header flow.Header
		err := operation.RetrieveHeader(blockID, &header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve block: %w", err)
		}

		// retrieve the current finalized state boundary
		var boundary uint64
		err = operation.RetrieveBoundary(&boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve boundary: %w", err)
		}

		// retrieve the ID of the boundary head
		var headID flow.Identifier
		err = operation.RetrieveNumber(boundary, &headID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve head: %w", err)
		}

		// check that the head ID is the parent of the block we finalize
		if header.ParentID != headID {
			return fmt.Errorf("can't finalize non-child of chain head")
		}

		// insert the number to block mapping
		err = operation.InsertNumber(header.Number, header.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not insert number mapping: %w", err)
		}

		// update the finalized boundary
		err = operation.UpdateBoundary(header.Number)(tx)
		if err != nil {
			return fmt.Errorf("could not update finalized boundary: %w", err)
		}

		// NOTE: we don't want to prune forks that have become invalid here, so
		// that we can keep validating entities and generating slashing
		// challenges for some time - the pruning should happen some place else
		// after a certain delay of blocks

		return nil
	}
}

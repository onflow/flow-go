// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package procedure

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

func IndexGuarantees(height uint64, blockID flow.Identifier, parentID flow.Identifier, guarantees []*flow.CollectionGuarantee) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// check that all guarantees are part of the database
		for _, guarantee := range guarantees {
			var exists bool
			err := operation.CheckGuarantee(guarantee.CollectionID, &exists)(tx)
			if err != nil {
				return fmt.Errorf("could not check guarantee in DB (%x): %w", guarantee.CollectionID, err)
			}
			if !exists {
				return fmt.Errorf("node guarantee missing in DB (%x)", guarantee.CollectionID)
			}
		}

		// insert the list of IDs into the payload index
		err := operation.IndexGuaranteePayload(height, blockID, parentID, flow.GetIDs(guarantees))(tx)
		if err != nil {
			return fmt.Errorf("could not index guarantees: %w", err)
		}

		return nil
	}
}

func RetrieveGuarantees(blockID flow.Identifier, guarantees *[]*flow.CollectionGuarantee) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// get the header so we have the height
		var header flow.Header
		err := operation.RetrieveHeader(blockID, &header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header: %w", err)
		}

		// get the collection IDs for the guarantees
		var collIDs []flow.Identifier
		err = operation.LookupGuaranteePayload(header.Height, blockID, header.ParentID, &collIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not lookup guarantees: %w", err)
		}

		// return if there are no collections
		if len(collIDs) == 0 {
			return nil
		}

		// get all guarantees
		*guarantees = make([]*flow.CollectionGuarantee, 0, len(collIDs))
		for _, collID := range collIDs {
			var guarantee flow.CollectionGuarantee
			err = operation.RetrieveGuarantee(collID, &guarantee)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve guarantee (%x): %w", collID, err)
			}
			*guarantees = append(*guarantees, &guarantee)
		}

		return nil
	}
}

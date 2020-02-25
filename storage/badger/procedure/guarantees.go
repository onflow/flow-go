// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package procedure

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

func IndexGuarantees(payloadHash flow.Identifier, guarantees []*flow.CollectionGuarantee) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// check and index the guarantees
		for i, guarantee := range guarantees {
			var exists bool
			err := operation.CheckGuarantee(guarantee.CollectionID, &exists)(tx)
			if err != nil {
				return fmt.Errorf("could not check guarantee in DB (%x): %w", guarantee.CollectionID, err)
			}
			if !exists {
				return fmt.Errorf("node guarantee missing in DB (%x)", guarantee.CollectionID)
			}

			// TODO: Revisit duplicate handling logic
			err = operation.AllowDuplicates(operation.IndexGuarantee(payloadHash, uint64(i), guarantee.CollectionID))(tx)
			if err != nil {
				return fmt.Errorf("could not index guarantee (%x): %w", guarantee.CollectionID, err)
			}
		}

		return nil
	}
}

func RetrieveGuarantees(payloadHash flow.Identifier, guarantees *[]*flow.CollectionGuarantee) func(*badger.Txn) error {

	return func(tx *badger.Txn) error {

		var retrievedGuarantees []*flow.CollectionGuarantee

		// get the collection IDs for the guarantees
		var collIDs []flow.Identifier
		err := operation.LookupGuarantees(payloadHash, &collIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not lookup guarantees: %w", err)
		}

		// get all guarantees
		for _, collID := range collIDs {
			var guarantee flow.CollectionGuarantee
			err = operation.RetrieveGuarantee(collID, &guarantee)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve guarantee (%x): %w", collID, err)
			}
			retrievedGuarantees = append(retrievedGuarantees, &guarantee)
		}

		*guarantees = retrievedGuarantees

		return nil
	}
}

// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package procedure

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

func IndexSeals(blockID flow.Identifier, sealIDs []flow.Identifier) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// check that all seals are in the database
		for _, sealID := range sealIDs {
			var exists bool
			err := operation.CheckSeal(sealID, &exists)(tx)
			if err != nil {
				return fmt.Errorf("could not check seal in DB (%x): %w", sealID, err)
			}
			if !exists {
				return fmt.Errorf("node seal missing in DB (%x)", sealID)
			}
		}

		// insert the list of IDs into the payload index
		err := operation.IndexSealPayload(blockID, sealIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not index seals: %w", err)
		}

		return nil
	}
}

func RetrieveSeals(blockID flow.Identifier, seals *[]*flow.Seal) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// get the header so we have the height
		var header flow.Header
		err := operation.RetrieveHeader(blockID, &header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header: %w", err)
		}

		// get the sealection IDs for the seals
		var sealIDs []flow.Identifier
		err = operation.LookupSealPayload(blockID, &sealIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not lookup seals: %w", err)
		}

		// return if there are no seals
		if len(sealIDs) == 0 {
			return nil
		}

		// get all seals
		*seals = make([]*flow.Seal, 0, len(sealIDs))
		for _, sealID := range sealIDs {
			var seal flow.Seal
			err = operation.RetrieveSeal(sealID, &seal)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve seal (%x): %w", sealID, err)
			}
			*seals = append(*seals, &seal)
		}

		return nil
	}
}

// LookupSealByBlock retrieves seal by block for which it was the highest seal.
func LookupSealByBlock(blockID flow.Identifier, seal *flow.Seal) func(*badger.Txn) error {

	return func(tx *badger.Txn) error {

		var sealID flow.Identifier

		err := operation.LookupSealIDByBlock(blockID, &sealID)(tx)
		if err != nil {
			return fmt.Errorf("could not lookup seal ID by block: %w", err)
		}

		err = operation.RetrieveSeal(sealID, seal)(tx)
		if err != nil {
			return fmt.Errorf("coulnd not retrieve seal for sealID (%x): %w", sealID, err)
		}
		return nil
	}
}

// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package procedure

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

func IndexSeals(payloadHash flow.Identifier, seals []*flow.Seal) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// check and index the seals
		for i, seal := range seals {
			var exists bool
			err := operation.CheckSeal(seal.ID(), &exists)(tx)
			if err != nil {
				return fmt.Errorf("could not check seal in DB (%x): %w", seal.ID(), err)
			}
			if !exists {
				return fmt.Errorf("node seal missing in DB (%x)", seal.ID())
			}
			err = operation.IndexSeal(payloadHash, uint64(i), seal.ID())(tx)
			if err != nil {
				return fmt.Errorf("could not index seal (%x): %w", seal.ID(), err)
			}
		}

		return nil
	}
}

func RetrieveSeals(payloadHash flow.Identifier, seals *[]*flow.Seal) func(*badger.Txn) error {

	// make sure we have a zero value
	//*seals = make([]*flow.Seal, 0)


	return func(tx *badger.Txn) error {

		var retrievedSeals []*flow.Seal

		// get the sealection IDs for the seals
		var sealIDs []flow.Identifier
		err := operation.LookupSeals(payloadHash, &sealIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not lookup seals: %w", err)
		}

		// get all seals
		//*seals = make([]*flow.Seal, 0, len(sealIDs))
		for _, sealID := range sealIDs {
			var seal flow.Seal
			err = operation.RetrieveSeal(sealID, &seal)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve seal (%x): %w", sealID, err)
			}
			retrievedSeals = append(retrievedSeals, &seal)
		}

		*seals = retrievedSeals

		return nil
	}
}

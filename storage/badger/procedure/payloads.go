// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package procedure

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

func InsertPayload(blockID flow.Identifier, payload *flow.Payload) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// insert the block guarantees
		for _, guarantee := range payload.Guarantees {
			err := operation.SkipDuplicates(operation.InsertGuarantee(guarantee.ID(), guarantee))(tx)
			if err != nil {
				return fmt.Errorf("could not insert guarantee (%x): %w", guarantee.CollectionID, err)
			}
		}

		// insert the block seals
		for _, seal := range payload.Seals {
			err := operation.SkipDuplicates(operation.InsertSeal(seal.ID(), seal))(tx)
			if err != nil {
				return fmt.Errorf("could not insert seal (%x): %w", seal.ID(), err)
			}
		}

		// index guarantees
		err := operation.IndexPayloadGuarantees(blockID, flow.GetIDs(payload.Guarantees))(tx)
		if err != nil {
			return fmt.Errorf("could not index guarantees: %w", err)
		}

		// index seals
		err = operation.IndexPayloadSeals(blockID, flow.GetIDs(payload.Seals))(tx)
		if err != nil {
			return fmt.Errorf("could not index seals: %w", err)
		}

		return nil
	}
}

func RetrievePayload(blockID flow.Identifier, payload *flow.Payload) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {

		// retrieve the guarantee IDs
		var guarIDs []flow.Identifier
		err := operation.LookupPayloadGuarantees(blockID, &guarIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not lookup guarantees for block: %w", err)
		}

		// retrieve the seal IDs
		var sealIDs []flow.Identifier
		err = operation.LookupPayloadSeals(blockID, &sealIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not lookup seals for block: %w", err)
		}

		// get guarantees
		guarantees := make([]*flow.CollectionGuarantee, 0, len(guarIDs))
		for _, guarID := range guarIDs {
			var guarantee flow.CollectionGuarantee
			err := operation.RetrieveGuarantee(guarID, &guarantee)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve guarantees: %w", err)
			}
			guarantees = append(guarantees, &guarantee)
		}

		// get seals
		seals := make([]*flow.Seal, 0, len(sealIDs))
		for _, sealID := range sealIDs {
			var seal flow.Seal
			err := operation.RetrieveSeal(sealID, &seal)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve seals: %w", err)
			}
			seals = append(seals, &seal)
		}

		// create the block content
		*payload = flow.Payload{
			Identities: nil,
			Guarantees: guarantees,
			Seals:      seals,
		}

		return nil
	}
}

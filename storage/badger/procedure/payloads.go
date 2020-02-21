// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package procedure

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

func InsertPayload(payload *flow.Payload) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// insert the block identities
		for _, identity := range payload.Identities {
			err := operation.AllowDuplicates(operation.InsertIdentity(identity))(tx)
			if err != nil {
				return fmt.Errorf("could not insert identity (%x): %w", identity.NodeID, err)
			}
		}

		// insert the block guarantees
		for _, guarantee := range payload.Guarantees {
			err := operation.AllowDuplicates(operation.InsertGuarantee(guarantee))(tx)
			if err != nil {
				return fmt.Errorf("could not insert guarantee (%x): %w", guarantee.CollectionID, err)
			}
		}

		// insert the block seals
		for _, seal := range payload.Seals {
			err := operation.AllowDuplicates(operation.InsertSeal(seal))(tx)
			if err != nil {
				return fmt.Errorf("could not insert seal (%x): %w", seal.ID(), err)
			}
		}

		return nil
	}
}

func IndexPayload(payload *flow.Payload) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// index identities
		err := IndexIdentities(payload.Hash(), payload.Identities)(tx)
		if err != nil {
			return fmt.Errorf("could not index identities: %w", err)
		}

		// index guarantees
		err = IndexGuarantees(payload.Hash(), payload.Guarantees)(tx)
		if err != nil {
			return fmt.Errorf("could not index guarantees: %w", err)
		}

		// index seals
		err = IndexSeals(payload.Hash(), payload.Seals)(tx)
		if err != nil {
			return fmt.Errorf("could not index seals: %w", err)
		}

		return nil
	}
}

func RetrievePayload(payloadHash flow.Identifier, payload *flow.Payload) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {

		// make sure there is a nil value on error
		*payload = flow.Payload{}

		// get identities
		var identities []*flow.Identity
		err := RetrieveIdentities(payloadHash, &identities)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve identities: %w", err)
		}

		// get guarantees
		var guarantees []*flow.CollectionGuarantee
		err = RetrieveGuarantees(payloadHash, &guarantees)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve guarantees: %w", err)
		}

		// get seals
		var seals []*flow.Seal
		err = RetrieveSeals(payloadHash, &seals)(tx)

		if err != nil {
			return fmt.Errorf("could not retrieve seals: %w", err)
		}

		// create the block content
		*payload = flow.Payload{
			Identities: identities,
			Guarantees: guarantees,
			Seals:      seals,
		}

		return nil
	}
}

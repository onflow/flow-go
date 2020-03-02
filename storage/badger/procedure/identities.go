// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package procedure

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

func IndexIdentities(payloadHash flow.Identifier, identities []*flow.Identity) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// check and index the identities
		for i, identity := range identities {
			var exists bool
			err := operation.CheckIdentity(identity.NodeID, &exists)(tx)
			if err != nil {
				return fmt.Errorf("could not check identity in DB (%x): %w", identity.NodeID, err)
			}
			if !exists {
				return fmt.Errorf("node identity missing in DB (%x)", identity.NodeID)
			}
			err = operation.IndexIdentity(payloadHash, uint64(i), identity.NodeID)(tx)
			if err != nil {
				return fmt.Errorf("could not index identity (%x): %w", identity.NodeID, err)
			}
		}

		return nil
	}
}

func RetrieveIdentities(payloadHash flow.Identifier, identities *[]*flow.Identity) func(*badger.Txn) error {

	return func(tx *badger.Txn) error {

		var retrievedIdentities []*flow.Identity

		// get the collection IDs for the identities
		var nodeIDs []flow.Identifier
		err := operation.LookupIdentities(payloadHash, &nodeIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not lookup identities: %w", err)
		}

		// get all identities
		for _, nodeID := range nodeIDs {
			var identity flow.Identity
			err = operation.RetrieveIdentity(nodeID, &identity)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve identity (%x): %w", nodeID, err)
			}
			retrievedIdentities = append(retrievedIdentities, &identity)
		}

		*identities = retrievedIdentities

		return nil
	}
}

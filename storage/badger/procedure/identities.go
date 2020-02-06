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
		for _, identity := range identities {
			var exists bool
			err := operation.CheckIdentity(identity.NodeID, &exists)(tx)
			if err != nil {
				return fmt.Errorf("could not check identity in DB (%x): %w", identity.NodeID, err)
			}
			if !exists {
				return fmt.Errorf("node identity missing in DB (%x)", identity.NodeID)
			}
			err = operation.IndexIdentity(payloadHash, identity.NodeID)(tx)
			if err != nil {
				return fmt.Errorf("could not index identity (%x): %w", identity.NodeID, err)
			}
		}

		return nil
	}
}

func RetrieveIdentities(payloadHash flow.Identifier, identities *[]*flow.Identity) func(*badger.Txn) error {

	// make sure we have a zero value
	*identities = make([]*flow.Identity, 0)

	return func(tx *badger.Txn) error {

		// get the collection IDs for the identities
		var nodeIDs []flow.Identifier
		err := operation.LookupIdentities(payloadHash, &nodeIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not lookup identities: %w", err)
		}

		// get all identities
		*identities = make([]*flow.Identity, 0, len(nodeIDs))
		for _, nodeID := range nodeIDs {
			var identity flow.Identity
			err = operation.RetrieveIdentity(nodeID, &identity)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve identity (%x): %w", nodeID, err)
			}
			*identities = append(*identities, &identity)
		}

		return nil
	}
}

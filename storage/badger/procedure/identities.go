// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package procedure

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

func IndexIdentities(height uint64, blockID flow.Identifier, parentID flow.Identifier, identities []*flow.Identity) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// check that all identities are part of the database
		for _, identity := range identities {
			var exists bool
			err := operation.CheckIdentity(identity.NodeID, &exists)(tx)
			if err != nil {
				return fmt.Errorf("could not check identity in DB (%x): %w", identity.NodeID, err)
			}
			if !exists {
				return fmt.Errorf("node identity missing in DB (%x)", identity.NodeID)
			}
		}

		// insert list of IDs into the payload index
		err := operation.IndexIdentityPayload(height, blockID, parentID, flow.GetIDs(identities))(tx)
		if err != nil {
			return fmt.Errorf("could not index identities: %w", err)
		}

		return nil
	}
}

func RetrieveIdentities(blockID flow.Identifier, identities *[]*flow.Identity) func(*badger.Txn) error {

	return func(tx *badger.Txn) error {

		// get the header so we have the height
		var header flow.Header
		err := operation.RetrieveHeader(blockID, &header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header: %w", err)
		}

		// get the nodeIDs for the identities
		var nodeIDs []flow.Identifier
		err = operation.LookupIdentityPayload(header.Height, blockID, header.ParentID, &nodeIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not lookup identities: %w", err)
		}

		// return if there are no identities
		if len(nodeIDs) == 0 {
			return nil
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

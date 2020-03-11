// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package procedure

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

func ApplyDeltas(number uint64, identities []*flow.Identity) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		// for now, deltas are only for the genesis identities, so just insert them
		for _, identity := range identities {
			err := operation.InsertDelta(number, identity.Role, identity.NodeID, int64(identity.Stake))(tx)
			if err != nil {
				return fmt.Errorf("could not insert delta (%x): %w", identity.NodeID, err)
			}
		}

		return nil
	}
}

package testutil

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

func UncheckedState(db *badger.DB, identities flow.IdentityList) (*protocol.State, error) {

	genesis := flow.Genesis(identities)

	// insert the block payload
	err := db.Update(func(tx *badger.Txn) error {
		err := procedure.InsertPayload(&genesis.Payload)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis payload: %w", err)
		}

		// apply the stake deltas
		err = procedure.ApplyDeltas(genesis.Number, genesis.Identities)(tx)
		if err != nil {
			return fmt.Errorf("could not apply stake deltas: %w", err)
		}

		// get first seal
		seal := genesis.Seals[0]

		// insert the block seal commit
		err = operation.InsertCommit(genesis.ID(), seal.FinalState)(tx)
		if err != nil {
			return fmt.Errorf("could not insert state commit: %w", err)
		}

		// index the block seal commit
		err = operation.IndexCommit(genesis.ID(), seal.FinalState)(tx)
		if err != nil {
			return fmt.Errorf("could not index state commit: %w", err)
		}

		// insert the genesis block
		err = procedure.InsertBlock(genesis)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis block: %w", err)
		}

		// insert the block number mapping
		err = operation.InsertNumber(0, genesis.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not initialize boundary: %w", err)
		}

		// insert the finalized boundary
		err = operation.InsertBoundary(genesis.Number)(tx)
		if err != nil {
			return fmt.Errorf("could not update boundary: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not initialize database: %w", err)
	}

	state, err := protocol.NewState(db)
	if err != nil {
		return nil, fmt.Errorf("could not initialize state: %w", err)
	}

	return state, nil
}

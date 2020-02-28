package testutil

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// ExecutionStateBootstrap performs execution node specific bootstrap
func ExecutionStateBootstrap(tx *badger.Txn, genesis *flow.Block) error {

	// insert the block seal commit
	err := operation.IndexCommit(genesis.ID(), genesis.Seals[0].FinalState)(tx)
	if err != nil {
		return fmt.Errorf("could not insert state commit: %w", err)
	}

	return nil
}

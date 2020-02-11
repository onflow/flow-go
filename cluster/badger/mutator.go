package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Mutator struct {
	state *State
}

func (m *Mutator) Bootstrap(genesis *cluster.Block) error {
	return m.state.db.Update(func(tx *badger.Txn) error {

		// check header number
		if genesis.Number != 0 {
			return fmt.Errorf("genesis number should be 0 (got %d)", genesis.Number)
		}

		// check header parent ID
		if genesis.ParentID != flow.ZeroID {
			return fmt.Errorf("genesis parent ID must be zero hash (got %x)", genesis.ParentID)
		}

		// check payload
		collSize := len(genesis.Collection.Transactions)
		if collSize != 0 {
			return fmt.Errorf("genesis collection should contain no transactions (got %d)", collSize)
		}

		// insert payload
		err := operation.InsertCollection(genesis.Collection)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis collection: %w", err)
		}

		// index payload
		err = operation.IndexCollection(genesis.Payload.Hash(), genesis.Collection)(tx)
		if err != nil {
			return fmt.Errorf("could not index genesis collection: %w", err)
		}

		// insert header
		err = operation.InsertHeader(&genesis.Header)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis header: %w", err)
		}

		// insert block number -> ID mapping
		err = operation.InsertNumberForCluster(genesis.ChainID, genesis.Number, genesis.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis number: %w", err)
		}

		// insert boundary
		err = operation.InsertBoundaryForCluster(genesis.ChainID, genesis.Number)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis boundary: %w", err)
		}

		return nil
	})
}

func (m *Mutator) Extend(blockID flow.Identifier) error {
	panic("TODO")
}

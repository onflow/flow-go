package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
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
		// insert genesis block
		// insert block number mapping
		// insert boundary

		return nil
	})
}

func (m *Mutator) Extend(blockID flow.Identifier) error {
	panic("TODO")
}

package badger

import (
	"fmt"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// StateRoot is the root information required to bootstrap the cluster state
type StateRoot struct {
	block     *cluster.Block
	clusterID flow.ChainID
}

func NewStateRoot(clusterID flow.ChainID, genesis *cluster.Block) (*StateRoot, error) {
	err := validate(clusterID, genesis)
	if err != nil {
		return nil, fmt.Errorf("inconsistent state root: %w", err)
	}
	return &StateRoot{
		clusterID: clusterID,
		block:     genesis,
	}, nil
}

func validate(clusterID flow.ChainID, genesis *cluster.Block) error {
	// check chain ID
	if genesis.Header.ChainID != clusterID {
		return fmt.Errorf("genesis chain ID (%s) does not match configured (%s)", genesis.Header.ChainID, clusterID)
	}
	// check header number
	if genesis.Header.Height != 0 {
		return fmt.Errorf("genesis number should be 0 (got %d)", genesis.Header.Height)
	}
	// check header parent ID
	if genesis.Header.ParentID != flow.ZeroID {
		return fmt.Errorf("genesis parent ID must be zero hash (got %x)", genesis.Header.ParentID)
	}

	// check payload integrity
	if genesis.Header.PayloadHash != genesis.Payload.Hash() {
		return fmt.Errorf("computed payload hash does not match header")
	}

	// check payload
	collSize := len(genesis.Payload.Collection.Transactions)
	if collSize != 0 {
		return fmt.Errorf("genesis collection should contain no transactions (got %d)", collSize)
	}

	return nil
}

func (s StateRoot) ClusterID() flow.ChainID {
	return s.clusterID
}

func (s StateRoot) Block() *cluster.Block {
	return s.block
}

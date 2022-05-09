package badger

import (
	"fmt"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// StateRoot is the root information required to bootstrap the cluster state
type StateRoot struct {
	block *cluster.Block
	qc    *flow.QuorumCertificate
}

func NewStateRoot(genesis *cluster.Block, qc *flow.QuorumCertificate) (*StateRoot, error) {
	err := validateClusterGenesis(genesis)
	if err != nil {
		return nil, fmt.Errorf("inconsistent state root: %w", err)
	}
	return &StateRoot{
		block: genesis,
		qc:    qc,
	}, nil
}

func validateClusterGenesis(genesis *cluster.Block) error {
	// check height of genesis block
	if genesis.Header.Height != 0 {
		return fmt.Errorf("height of genesis cluster block should be 0 (got %d)", genesis.Header.Height)
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
	return s.block.Header.ChainID
}

func (s StateRoot) Block() *cluster.Block {
	return s.block
}

func (s StateRoot) QC() *flow.QuorumCertificate {
	return s.qc
}

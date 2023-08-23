package module

import (
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

type Indexer interface {
	Last() (uint64, error)                                                    // get last indexed sealed block height
	HeightForBlock(ID flow.Identifier) (uint64, error)                        // get height by block ID
	Commitment(height uint64) (flow.StateCommitment, error)                   // get state commitment by height
	Values(height uint64, IDs flow.RegisterIDs) ([]flow.RegisterValue, error) // retrieve register values for register IDs at given height

	StorePayloads(height uint64, payloads []*ledger.Payload) error // index payloads at given height
}

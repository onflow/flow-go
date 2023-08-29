package module

import (
	"github.com/onflow/flow-go/engine/execution/scripts"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

type ExecutionStateIndexer interface {
	ExecutionStateIndexReader
	ExecutionStateIndexWriter
	scripts.ScriptExecutionState
}

type ExecutionStateIndexReader interface {
	// Last returns the last block height that was indexed.
	//
	// Expect an error if no value exists.
	Last() (uint64, error)
	// HeightByBlockID returns the block height by block ID.
	//
	// Expect an error if block by the ID was not indexed.
	HeightByBlockID(ID flow.Identifier) (uint64, error)
	// Commitment by the block height.
	//
	// Expect an error if commitment by block height was not indexed.
	Commitment(height uint64) (flow.StateCommitment, error)
	// Values retrieve register values for register IDs at given block height or by
	// the block height that is highest but still lower than the provided height.
	//
	// Expect an error if register was not indexed at all.
	Values(IDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error)
}

type ExecutionStateIndexWriter interface {
	// StorePayloads at the provided block height.
	//
	// Expect an error if we are trying to overwrite existing payloads at the provided height.
	StorePayloads(payloads []*ledger.Payload, height uint64) error
	// StoreCommitment at the provided block height.
	//
	// Expect an error if we are trying to overwrite existing commitments at provided height.
	StoreCommitment(commitment flow.StateCommitment, height uint64) error
	// StoreLast block height that was indexed.
	// Normal operation shouldn't produce an error.
	StoreLast(uint64) error
}

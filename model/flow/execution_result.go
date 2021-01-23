package flow

import (
	"github.com/onflow/flow-go/crypto"
)

// ExecutionResultBody ...
type ExecutionResultBody struct {
	PreviousResultID Identifier // commit of the previous ER
	BlockID          Identifier // commit of the current block
	Chunks           ChunkList
}

// ExecutionResult ...
type ExecutionResult struct {
	ExecutionResultBody

	// TODO: this is executor specific and should move into the ExecutionReceipt
	Signatures []crypto.Signature
}

// ID returns the hash of the execution result body
func (er ExecutionResult) ID() Identifier {
	return MakeID(er.ExecutionResultBody)
}

// Checksum ...
func (er ExecutionResult) Checksum() Identifier {
	return MakeID(er)
}

// FinalStateCommitment returns the Execution Result's commitment to the final
// execution state of the block, i.e. the last chunk's output state.
//
// By protocol definition, each ExecutionReceipt must contain at least one
// chunk (system chunk). Convention: publishing an ExecutionReceipt without a
// final state commitment is a slashable protocol violation.
// TODO: change bool to error return with a sentinel error
func (er ExecutionResult) FinalStateCommitment() (StateCommitment, bool) {
	if er.Chunks.Len() == 0 {
		return nil, false
	}
	s := er.Chunks[er.Chunks.Len()-1].EndState
	return s, len(s) > 0
}

// InitialStateCommit returns a commitment to the execution state used as input
// for computing the block the block, i.e. the leading chunk's input state.
//
// By protocol definition, each ExecutionReceipt must contain at least one
// chunk (system chunk). Convention: publishing an ExecutionReceipt without an
// initial state commitment is a slashable protocol violation.
// TODO: change bool to error return with a sentinel error
func (er ExecutionResult) InitialStateCommit() (StateCommitment, bool) {
	if er.Chunks.Len() == 0 {
		return nil, false
	}
	s := er.Chunks[0].StartState
	return s, len(s) > 0
}

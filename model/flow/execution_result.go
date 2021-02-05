package flow

import "github.com/dapperlabs/flow-go/model/flow"

// ExecutionResult ...
type ExecutionResult struct {
	PreviousResultID Identifier // commit of the previous ER
	BlockID          Identifier // commit of the current block
	Chunks           ChunkList
	ServiceEvents    []Event
}

// ID returns the hash of the execution result body
func (er ExecutionResult) ID() Identifier {
	return MakeID(er)
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

// IsSystemChunk returns true if `chunkIndex` points to a system chunk in `result`.
// Otherwise, it returns false.
// In the current version, a chunk is a system chunk if it is the last chunk of the
// execution result.
func IsSystemChunk(chunkIndex uint64, result *flow.ExecutionResult) bool {
	return chunkIndex == uint64(len(result.Chunks)-1)
}

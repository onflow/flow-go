package flow

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

// ValidateChunksLength checks whether the number of chuncks is zero.
//
// It returns false if the number of chunks is zero (invalid).
// By protocol definition, each ExecutionReceipt must contain at least one
// chunk (system chunk).
func (er ExecutionResult) ValidateChunksLength() bool {
	return er.Chunks.Len() != 0
}

// FinalStateCommitment returns the Execution Result's commitment to the final
// execution state of the block, i.e. the last chunk's output state.
//
// The function assumes the number of chunks is non-zero. A check using
// ValidateChunksLength is required before calling this function.
func (er ExecutionResult) FinalStateCommitment() StateCommitment {
	return er.Chunks[er.Chunks.Len()-1].EndState
}

// InitialStateCommit returns a commitment to the execution state used as input
// for computing the block the block, i.e. the leading chunk's input state.
//
// The function assumes the number of chunks is non-zero. A check using
// ValidateChunksLength is required before calling this function.
func (er ExecutionResult) InitialStateCommit() StateCommitment {
	return er.Chunks[0].StartState
}

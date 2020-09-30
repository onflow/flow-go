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

// FinalStateCommitment gets the final state of the result and returns false
// if the number of chunks is 0 (used as a sanity check)
func (er ExecutionResult) FinalStateCommitment() (StateCommitment, bool) {
	if er.Chunks.Len() == 0 {
		return nil, false
	}

	return er.Chunks[er.Chunks.Len()-1].EndState, true
}

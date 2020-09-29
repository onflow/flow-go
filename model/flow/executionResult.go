package flow

import (
	"github.com/onflow/flow-go/crypto"
)

type ExecutionResultBody struct {
	PreviousResultID Identifier // commit of the previous ER
	BlockID          Identifier // commit of the current block
	Chunks           ChunkList
}

type ExecutionResult struct {
	ExecutionResultBody
	Signatures []crypto.Signature
}

func (er ExecutionResult) ID() Identifier {
	return MakeID(er.ExecutionResultBody)
}

func (er ExecutionResult) Checksum() Identifier {
	return MakeID(er)
}

// FinalStateCommit gets the final state of the result
// if the number of chunks are 0 returns empty statecommit
func (er ExecutionResult) FinalStateCommit() StateCommitment {
	if er.Chunks.Len() == 0 {
		return StateCommitment{}
	}

	return er.Chunks[er.Chunks.Len()-1].EndState
}

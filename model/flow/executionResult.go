package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
)

type ExecutionResultBody struct {
	PreviousResultID Identifier      // commit of the previous ER
	BlockID          Identifier      // commit of the current block
	FinalStateCommit StateCommitment // final state commitment
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
